// Package server wires bounded contexts into one HTTP handler (modular monolith).
package server

import (
	"log/slog"
	"net/http"
	"time"

	issuehttp "github.com/bakhod1r/kyber/internal/issue/adapter/httpapi"
	issuememory "github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	"github.com/bakhod1r/kyber/internal/issue/adapter/projectacl"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/events"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/id"
	projecthttp "github.com/bakhod1r/kyber/internal/project/adapter/httpapi"
	projectmemory "github.com/bakhod1r/kyber/internal/project/adapter/memory"
	projectapp "github.com/bakhod1r/kyber/internal/project/app"
)

// NewInMemory builds the API backed by in-memory adapters (Sprint 01; Postgres follows).
func NewInMemory(log *slog.Logger) http.Handler {
	projects := projectapp.NewService(projectmemory.NewRepository(), id.New)
	issues := issueapp.NewService(
		issuememory.NewRepository(),
		projectacl.New(projects),
		events.LogPublisher[issuedomain.Event]{Log: log},
		issuedomain.DefaultWorkflow(),
		id.New,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	projecthttp.New(projects, log).Register(mux)
	issuehttp.New(issues, log).Register(mux)
	return logging(log, recoverer(log, mux))
}

func logging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "dur", time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.ErrorContext(r.Context(), "panic", "value", v, "path", r.URL.Path)
				httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
