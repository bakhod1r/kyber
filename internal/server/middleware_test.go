package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverReturns500OnPanic(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := recoverer(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("db password=hunter2") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	// Regression (review): the panic value is logged, never sent to the client.
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("panic leaked to client: %s", rec.Body)
	}
}

// S81: webhook secrets are in the URL; logs must never carry them.
func TestLoggedPathHidesWebhookSecrets(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/hooks/s3cr3t-token":  "/api/v1/hooks/…",
		"/api/v1/hooks/s3cr3t/extra":  "/api/v1/hooks/…",
		"/api/v1/conversations/c-1":   "/api/v1/conversations/c-1",
		"/api/v1/webhooks/w-1/rotate": "/api/v1/webhooks/w-1/rotate",
	} {
		if got := loggedPath(in); got != want {
			t.Fatalf("loggedPath(%q) = %q, want %q", in, got, want)
		}
	}
}
