// Package server wires bounded contexts into one HTTP handler (modular monolith).
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/identity/adapter/argon2"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/adapter/httpapi"
	identitymemory "github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	identitypg "github.com/bakhod1r/kyber/internal/identity/adapter/postgres"
	identityapp "github.com/bakhod1r/kyber/internal/identity/app"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	issuehttp "github.com/bakhod1r/kyber/internal/issue/adapter/httpapi"
	issuememory "github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	issuepg "github.com/bakhod1r/kyber/internal/issue/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/issue/adapter/projectacl"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/metrics"
	projecthttp "github.com/bakhod1r/kyber/internal/project/adapter/httpapi"
	"github.com/bakhod1r/kyber/internal/project/adapter/identitydir"
	projectmemory "github.com/bakhod1r/kyber/internal/project/adapter/memory"
	projectpg "github.com/bakhod1r/kyber/internal/project/adapter/postgres"
	projectapp "github.com/bakhod1r/kyber/internal/project/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/web"
)

// deps are the adapters a server is assembled from.
type deps struct {
	projects     projectdomain.Repository
	issues       issuedomain.Repository
	users        identitydomain.Users
	sessions     identitydomain.Sessions
	ready        func(context.Context) error
	cookieSecure bool
}

// NewInMemory builds the API on in-memory adapters (dev mode and tests).
func NewInMemory(log *slog.Logger) http.Handler {
	ids := identitymemory.NewRepository()
	return build(log, deps{
		projects: projectmemory.NewRepository(), issues: issuememory.NewRepository(),
		users: ids, sessions: ids, ready: func(context.Context) error { return nil },
	})
}

// StartJobs runs background maintenance (hourly expired-session purge) until ctx ends.
func StartJobs(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool) {
	ids := identitypg.NewRepository(pool)
	svc := identityapp.NewService(ids, ids, argon2.New(), systemClock{}, id.New)
	go svc.RunSessionPurger(ctx, time.Hour, func(n int, err error) {
		if err != nil {
			log.ErrorContext(ctx, "session purge failed", "err", err)
			return
		}
		log.InfoContext(ctx, "expired sessions purged", "count", n)
	})
}

// NewPostgres builds the API on PostgreSQL; the pool must already be migrated.
func NewPostgres(log *slog.Logger, pool *pgxpool.Pool, cookieSecure bool) http.Handler {
	ids := identitypg.NewRepository(pool)
	return build(log, deps{
		projects: projectpg.NewRepository(pool), issues: issuepg.NewRepository(pool),
		users: ids, sessions: ids, ready: pool.Ping, cookieSecure: cookieSecure,
	})
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func build(log *slog.Logger, d deps) http.Handler {
	projects := projectapp.NewService(d.projects, identitydir.New(d.users), id.New)
	acl := projectacl.New(projects)
	issues := issueapp.NewService(d.issues, acl, acl, issuedomain.DefaultWorkflow(), id.New)
	identity := identityapp.NewService(d.users, d.sessions, argon2.New(), systemClock{}, id.New)
	auth := identityhttp.New(identity, log, d.cookieSecure)
	m := metrics.NewHTTP()

	api := http.NewServeMux()
	auth.RegisterProtected(api)
	projecthttp.New(projects, log).Register(api)
	issuehttp.New(issues, log).Register(api)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.ready(ctx); err != nil {
			log.WarnContext(r.Context(), "not ready", "err", err)
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", m)
	auth.RegisterPublic(mux)
	mux.Handle("/api/v1/", auth.RequireAuth(api))
	mux.Handle("/", web.Handler(web.Dist()))
	return observe(log, m, recoverer(log, mux))
}

func observe(log *slog.Logger, m *metrics.HTTP, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		d := time.Since(start)
		m.Observe(r.Method, rec.status, d)
		log.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", d)
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
