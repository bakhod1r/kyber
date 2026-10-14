// Package llm summarizes huddle transcripts with an OpenAI-compatible chat model (POST {url} with
// {model, messages} → choices[0].message.content), e.g. a self-hosted Ollama or vLLM server. Used only when
// KYBER_LLM_URL is set: otherwise transcripts never leave the server (S87).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MaxInput caps the transcript sent to the model (in characters).
const MaxInput = 16000

const prompt = "Summarize this meeting transcript in 3-6 short bullet points: decisions, action items with owners, " +
	"open questions. Answer in the transcript's language. Do not invent anything."

type Client struct {
	url, model, key string
	http            *http.Client
}

// New: url is the chat completions endpoint (e.g. http://ollama:11434/v1/chat/completions); key, when set, is sent
// as a bearer token.
func New(url, model, key string) *Client {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{url: url, model: model, key: key, http: &http.Client{Timeout: 5 * time.Minute}}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Client) Summarize(ctx context.Context, transcript string) (string, error) {
	if r := []rune(transcript); len(r) > MaxInput {
		transcript = string(r[:MaxInput])
	}
	body, err := json.Marshal(map[string]any{"model": c.model, "messages": []message{{"system", prompt}, {"user", transcript}}})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat model: %s", res.Status)
	}
	var out struct {
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("chat model: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("chat model: no answer")
	}
	return out.Choices[0].Message.Content, nil
}
