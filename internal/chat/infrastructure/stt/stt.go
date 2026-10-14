// Package stt transcribes huddle recordings with an OpenAI-compatible speech-to-text service
// (POST {url} multipart: file, model → {"text": …}), e.g. a self-hosted Whisper server. Used only when
// KYBER_STT_URL is set: otherwise recordings never leave the server (S77).
package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	url, model, key string
	http            *http.Client
}

// New: url is the transcription endpoint (e.g. http://whisper:8000/v1/audio/transcriptions); key, when set, is
// sent as a bearer token.
func New(url, model, key string) *Client {
	if model == "" {
		model = "whisper-1"
	}
	return &Client{url: url, model: model, key: key, http: &http.Client{Timeout: 30 * time.Minute}}
}

func (c *Client) Transcribe(ctx context.Context, name string, audio io.Reader) (string, error) {
	body, w := io.Pipe()
	form := multipart.NewWriter(w)
	go func() {
		part, err := form.CreateFormFile("file", name)
		if err == nil {
			_, err = io.Copy(part, audio)
		}
		if err == nil {
			err = form.WriteField("model", c.model)
		}
		if err == nil {
			err = form.Close()
		}
		_ = w.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("speech-to-text: %s", res.Status)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("speech-to-text: %w", err)
	}
	return out.Text, nil
}
