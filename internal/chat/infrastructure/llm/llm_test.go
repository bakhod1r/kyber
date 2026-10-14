package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/infrastructure/llm"
)

func TestSummarize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Model    string `json:"model"`
			Messages []struct{ Role, Content string }
		}
		if r.Header.Get("Authorization") != "Bearer k" || json.NewDecoder(r.Body).Decode(&in) != nil || in.Model != "gpt-4o-mini" ||
			len(in.Messages) != 2 || in.Messages[0].Role != "system" || in.Messages[1].Content != "we ship friday" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Ship on Friday."}}]}`))
	}))
	defer srv.Close()
	got, err := llm.New(srv.URL, "", "k").Summarize(context.Background(), "we ship friday")
	if err != nil || got != "Ship on Friday." {
		t.Fatalf("summarize = %q %v", got, err)
	}
	_, err = llm.New(srv.URL, "", "secret-key").Summarize(context.Background(), "we ship friday")
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("a failing model must surface without the key: %v", err)
	}
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"choices":[]}`)) }))
	defer empty.Close()
	if _, err := llm.New(empty.URL, "m", "").Summarize(context.Background(), "x"); err == nil {
		t.Fatal("no answer is an error")
	}
}

// Long transcripts are cut before they are sent.
func TestSummarizeCapsTheTranscript(t *testing.T) {
	var size int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Messages []struct{ Content string } }
		_ = json.NewDecoder(r.Body).Decode(&in)
		size = len([]rune(in.Messages[1].Content))
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	if _, err := llm.New(srv.URL, "m", "").Summarize(context.Background(), strings.Repeat("ж", llm.MaxInput+500)); err != nil || size != llm.MaxInput {
		t.Fatalf("sent %d runes, %v", size, err)
	}
}
