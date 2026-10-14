package stt_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/infrastructure/stt"
)

func TestTranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || r.FormValue("model") != "whisper-1" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil || h.Filename != "huddle.webm" {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(f)
		_, _ = io.WriteString(w, `{"text":"hello `+string(b)+`"}`)
	}))
	defer srv.Close()
	got, err := stt.New(srv.URL, "", "k").Transcribe(context.Background(), "huddle.webm", strings.NewReader("world"))
	if err != nil || got != "hello world" {
		t.Fatalf("transcribe = %q %v", got, err)
	}
	if _, err := stt.New(srv.URL, "", "wrong").Transcribe(context.Background(), "huddle.webm", strings.NewReader("x")); err == nil {
		t.Fatal("a failing service must surface (the relay retries)")
	}
}
