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
