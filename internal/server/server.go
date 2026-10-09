// Package server wires bounded contexts into one HTTP handler (modular monolith).
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/bakhod1r/errorx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	agilehttp "github.com/bakhod1r/kyber/internal/agile/adapter/httpapi"
	agilememory "github.com/bakhod1r/kyber/internal/agile/adapter/memory"
	agilepg "github.com/bakhod1r/kyber/internal/agile/adapter/postgres"
	agileprojectacl "github.com/bakhod1r/kyber/internal/agile/adapter/projectacl"
	agileapp "github.com/bakhod1r/kyber/internal/agile/app"
	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	calendaracl "github.com/bakhod1r/kyber/internal/calendar/adapter/acl"
	calendarhttp "github.com/bakhod1r/kyber/internal/calendar/adapter/httpapi"
	calendarmemory "github.com/bakhod1r/kyber/internal/calendar/adapter/memory"
	calendarpg "github.com/bakhod1r/kyber/internal/calendar/adapter/postgres"
	calendarapp "github.com/bakhod1r/kyber/internal/calendar/app"
	calendardomain "github.com/bakhod1r/kyber/internal/calendar/domain"
	focusacl "github.com/bakhod1r/kyber/internal/focus/adapter/acl"
	focushttp "github.com/bakhod1r/kyber/internal/focus/adapter/httpapi"
	focusmemory "github.com/bakhod1r/kyber/internal/focus/adapter/memory"
	focuspg "github.com/bakhod1r/kyber/internal/focus/adapter/postgres"
	focusapp "github.com/bakhod1r/kyber/internal/focus/app"
	focusdomain "github.com/bakhod1r/kyber/internal/focus/domain"
	"github.com/bakhod1r/kyber/internal/identity/adapter/argon2"
	"github.com/bakhod1r/kyber/internal/identity/adapter/guardlimit"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/adapter/httpapi"
	identitymemory "github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	identitypg "github.com/bakhod1r/kyber/internal/identity/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/identity/adapter/telegram"
	identityapp "github.com/bakhod1r/kyber/internal/identity/app"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	importeracl "github.com/bakhod1r/kyber/internal/importer/adapter/acl"
	importerhttp "github.com/bakhod1r/kyber/internal/importer/adapter/httpapi"
	importermemory "github.com/bakhod1r/kyber/internal/importer/adapter/memory"
	importerpg "github.com/bakhod1r/kyber/internal/importer/adapter/postgres"
	importerapp "github.com/bakhod1r/kyber/internal/importer/app"
	importerdomain "github.com/bakhod1r/kyber/internal/importer/domain"
	insightsacl "github.com/bakhod1r/kyber/internal/insights/adapter/acl"
	insightshttp "github.com/bakhod1r/kyber/internal/insights/adapter/httpapi"
	insightsmemory "github.com/bakhod1r/kyber/internal/insights/adapter/memory"
	insightspg "github.com/bakhod1r/kyber/internal/insights/adapter/postgres"
	insightsapp "github.com/bakhod1r/kyber/internal/insights/app"
	insightsdomain "github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/issue/adapter/agileacl"
	issuehttp "github.com/bakhod1r/kyber/internal/issue/adapter/httpapi"
	issuememory "github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	issuepg "github.com/bakhod1r/kyber/internal/issue/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/issue/adapter/projectacl"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	notifyacl "github.com/bakhod1r/kyber/internal/notify/adapter/acl"
	notifyhttp "github.com/bakhod1r/kyber/internal/notify/adapter/httpapi"
	notifymemory "github.com/bakhod1r/kyber/internal/notify/adapter/memory"
	notifypg "github.com/bakhod1r/kyber/internal/notify/adapter/postgres"
	notifyapp "github.com/bakhod1r/kyber/internal/notify/app"
	notifydomain "github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/metrics"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	projecthttp "github.com/bakhod1r/kyber/internal/project/adapter/httpapi"
	"github.com/bakhod1r/kyber/internal/project/adapter/identitydir"
	projectmemory "github.com/bakhod1r/kyber/internal/project/adapter/memory"
	projectpg "github.com/bakhod1r/kyber/internal/project/adapter/postgres"
	projectapp "github.com/bakhod1r/kyber/internal/project/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	workspacehttp "github.com/bakhod1r/kyber/internal/workspace/adapter/httpapi"
	workspacememory "github.com/bakhod1r/kyber/internal/workspace/adapter/memory"
	workspacepg "github.com/bakhod1r/kyber/internal/workspace/adapter/postgres"
	workspaceapp "github.com/bakhod1r/kyber/internal/workspace/app"
	workspacedomain "github.com/bakhod1r/kyber/internal/workspace/domain"
	"github.com/bakhod1r/kyber/web"
)

// deps are the adapters a server is assembled from.
type deps struct {
	projects      projectdomain.Repository
	issues        issuedomain.Repository
	comments      issuedomain.CommentRepository
	sprints       agiledomain.Repository
	users         identitydomain.Users
	external      identitydomain.ExternalIdentities
	social        identityhttp.Social
	otps          identitydomain.OTPChallenges
	telegramBot   *telegram.Bot
	telegramWait  time.Duration
	sessions      identitydomain.Sessions
	ready         func(context.Context) error
	cookieSecure  bool
	redis         redis.UniversalClient // optional: shared login limiter (guard)
	notes         notifydomain.Repository
	activity      insightsdomain.Repository
	mappings      importerdomain.MappingRepository
	workspaces    workspacedomain.Repository
	meetings      calendardomain.Repository
	focusSessions focusdomain.Repository
	baseDomain    string // "" = single-tenant (ADR-0004)
	outbox        outbox.Store
	ctx           context.Context // lifetime of background work (outbox relay)
	relayEvery    time.Duration
}

// WithContext bounds background work (the outbox relay) to ctx.
func WithContext(ctx context.Context) Option { return func(d *deps) { d.ctx = ctx } }

// WithRelayEvery sets how often the outbox relay polls (default 1s).
func WithRelayEvery(every time.Duration) Option { return func(d *deps) { d.relayEvery = every } }

// WithSocial enables sign-in with Google and/or Telegram.
func WithSocial(s identityhttp.Social) Option { return func(d *deps) { d.social = s } }

// WithTelegramBot lets the bot send one-time login codes; Kyber long-polls the bot's
// updates with the given wait (run one replica with the token, or a webhook later).
func WithTelegramBot(bot *telegram.Bot, wait time.Duration) Option {
	return func(d *deps) { d.telegramBot, d.telegramWait = bot, wait }
}

// WithBaseDomain enables workspaces on subdomains of domain (ADR-0004).
func WithBaseDomain(domain string) Option { return func(d *deps) { d.baseDomain = domain } }

// Option configures optional infrastructure.
type Option func(*deps)

// WithRedis shares the login limiter across replicas via guard's Redis limiter.
func WithRedis(rdb redis.UniversalClient) Option { return func(d *deps) { d.redis = rdb } }

// NewInMemory builds the API on in-memory adapters (dev mode and tests).
func NewInMemory(log *slog.Logger, opts ...Option) http.Handler {
	ids := identitymemory.NewRepository()
	issueRepo := issuememory.NewRepository()
	sprintRepo := agilememory.NewRepository()
	d := deps{
		projects: projectmemory.NewRepository(), issues: issueRepo, comments: issuememory.NewCommentRepository(issueRepo),
		sprints: sprintRepo, notes: notifymemory.NewRepository(), activity: insightsmemory.NewRepository(),
		mappings: importermemory.NewRepository(), workspaces: workspacememory.NewRepository(),
		meetings: calendarmemory.NewRepository(), focusSessions: focusmemory.NewRepository(),
		outbox: outbox.NewMemoryStore(events(issueRepo.Outbox, issueRepo.OutboxTimes), events(sprintRepo.Outbox, sprintRepo.OutboxTimes)),
		users:  ids, sessions: ids, external: ids, otps: ids, ready: func(context.Context) error { return nil },
	}
	for _, o := range opts {
		o(&d)
	}
	return build(log, d)
}

// events adapts a context's typed event log to the relay's generic one.
func events[E outbox.Event](log func() []E, times func() []time.Time) outbox.Source {
	return func() ([]outbox.Event, []time.Time) {
		at := times() // read first: never more times than events
		src := log()[:len(at)]
		out := make([]outbox.Event, len(src))
		for i, e := range src {
			out[i] = e
		}
		return out, at
	}
}

// StartJobs runs background maintenance (expired-session purge) until ctx ends.
func StartJobs(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, purgeEvery time.Duration) {
	ids := identitypg.NewRepository(pool)
	// The purge job also clears expired login-code challenges (no bot needed for that).
	svc := identityapp.NewService(ids, ids, argon2.New(), systemClock{}, id.New, identityapp.WithTelegramOTP(ids, nil))
	go svc.RunSessionPurger(ctx, purgeEvery, func(n int, err error) {
		if err != nil {
			log.ErrorContext(ctx, "session purge failed", "err", err)
			return
		}
		log.InfoContext(ctx, "expired sessions and login codes purged", "count", n)
	})
}

// NewPostgres builds the API on PostgreSQL; the pool must already be migrated.
func NewPostgres(log *slog.Logger, pool *pgxpool.Pool, cookieSecure bool, opts ...Option) http.Handler {
	ids := identitypg.NewRepository(pool)
	d := deps{
		projects: projectpg.NewRepository(pool), issues: issuepg.NewRepository(pool), comments: issuepg.NewCommentRepository(pool),
		sprints: agilepg.NewRepository(pool), notes: notifypg.NewRepository(pool), outbox: outbox.NewPostgresStore(pool),
		activity: insightspg.NewRepository(pool), mappings: importerpg.NewRepository(pool), workspaces: workspacepg.NewRepository(pool),
		meetings: calendarpg.NewRepository(pool), focusSessions: focuspg.NewRepository(pool),
		users: ids, sessions: ids, external: ids, otps: ids, ready: pool.Ping, cookieSecure: cookieSecure,
	}
	for _, o := range opts {
		o(&d)
	}
	if d.redis != nil {
		rdb := d.redis
		d.ready = func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				return err
			}
			return rdb.Ping(ctx).Err()
		}
	}
	return build(log, d)
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func build(log *slog.Logger, d deps) http.Handler {
	users := identitydir.New(d.users)
	workspaces := workspaceapp.NewService(d.workspaces, id.New)
	tenancy := workspacehttp.New(workspaces, log, d.baseDomain, d.cookieSecure)
	projects := projectapp.NewService(d.projects, users, id.New).WithWorkspaces(workspaces)
	acl := projectacl.New(projects, users)
	// Issue Tracking and Agile depend on each other through ports; the sprint
	// ACL is bound once both services exist.
	sprintACL := &lateSprints{}
	issues := issueapp.NewService(issueapp.Deps{
		Issues: d.issues, Comments: d.comments, Keys: acl, Access: acl, Directory: acl, Sprints: sprintACL,
		Workflow: issuedomain.DefaultWorkflow(), NewID: id.New, Now: time.Now,
	})
	agile := agileapp.NewService(agileapp.Deps{
		Sprints: d.sprints, Issues: issues, Access: agileprojectacl.New(projects), NewID: id.New, Now: time.Now,
	})
	sprintACL.Sprints = agileacl.New(agile)
	notify := notifyapp.NewService(notifyapp.Deps{
		Repo: d.notes, Issues: notifyacl.NewIssues(issues), Members: notifyacl.NewMembers(projects), Workspaces: notifyacl.NewWorkspaces(projects),
		Users: notifyacl.NewUsers(d.users), NewID: id.New, Now: time.Now, Log: log,
	})
	insights := insightsapp.NewService(insightsapp.Deps{
		Repo: d.activity, Access: insightsacl.NewAccess(projects), Issues: insightsacl.NewIssues(issues),
		Users: insightsacl.NewUsers(d.users), Sprints: insightsacl.NewSprints(agile), Now: time.Now, Log: log,
	})
	importer := importerapp.NewService(importerapp.Deps{
		Access: importeracl.NewProject(projects), Members: importeracl.NewProject(projects),
		Issues: importeracl.NewIssues(issues, time.Now), Mappings: d.mappings, Source: importeracl.NewSource(issues, d.users),
	})
	calendar := calendarapp.NewService(d.meetings, calendaracl.NewMembers(workspaces, d.users), id.New, time.Now)
	focus := focusapp.NewService(focusapp.Deps{Sessions: d.focusSessions, Calendar: focusacl.NewCalendar(calendar),
		Issues: focusacl.NewIssues(issues, projects), NewID: id.New, Clock: systemClock{}})
	relay := outbox.NewRelay(d.outbox, log)
	for _, name := range []string{"issue.created", "issue.transitioned", "issue.sprint_changed", "issue.estimated", "issue.imported"} {
		relay.Handle(name, insights.OnIssueEvent)
	}
	relay.Handle("issue.assigned", notify.OnIssueAssigned)
	relay.Handle("comment.added", notify.OnCommentAdded)
	if d.ctx == nil {
		d.ctx = context.Background()
	}
	if d.relayEvery == 0 {
		d.relayEvery = time.Second
	}
	go relay.Run(d.ctx, d.relayEvery)
	identityOpts := []identityapp.Option{identityapp.WithExternalIdentities(d.external)}
	if d.redis != nil {
		identityOpts = append(identityOpts, identityapp.WithLoginLimiter(guardlimit.New(d.redis, "kyber:")))
	}
	if d.telegramBot != nil {
		identityOpts = append(identityOpts, identityapp.WithTelegramOTP(d.otps, d.telegramBot))
		d.social.TelegramOTP = true
	}
	identity := identityapp.NewService(d.users, d.sessions, argon2.New(), systemClock{}, id.New, identityOpts...)
	if d.telegramBot != nil {
		go d.telegramBot.Poll(d.ctx, log, d.telegramWait, 5*time.Second,
			func(ctx context.Context, nonce string, from telegram.User, chat string) error {
				return identity.OnTelegramStart(ctx, nonce, strconv.FormatInt(from.ID, 10), chat, from.DisplayName())
			})
	}
	auth := identityhttp.New(identity, log, d.cookieSecure).WithSocial(d.social)
	m := metrics.NewHTTP()

	api := http.NewServeMux()
	auth.RegisterProtected(api)
	projecthttp.New(projects, log).Register(api)
	issuehttp.New(issues, log).Register(api)
	agilehttp.New(agile, log).Register(api)
	notifyhttp.New(notify, log).Register(api)
	insightshttp.New(insights, log).Register(api)
	importerhttp.New(importer, log).Register(api)
	tenancy.Register(api)
	calendarhttp.New(calendar, log, time.Now).Register(api)
	focushttp.New(focus, log, time.Now).Register(api)

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
	mux.Handle("/api/v1/", auth.RequireAuth(tenancy.RequireMember(problemFallback(api))))
	mux.Handle("/", web.Handler(web.Dist()))
	return observe(log, m, recoverer(log, tenancy.Resolve(mux)))
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
				log.ErrorContext(r.Context(), "panic", "value", fmt.Sprint(v), "path", r.URL.Path, "stack", string(debug.Stack()))
				httpx.Problem(w, r, errorx.New(httpx.CodeInternal, "internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// lateSprints breaks the construction cycle between Issue Tracking and Agile.
type lateSprints struct{ *agileacl.Sprints }

// problemFallback turns the router's plain-text 404/405 into RFC 9457 problems, so every API
// error has the same shape (the Allow header of a 405 is kept).
func problemFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&fallbackWriter{ResponseWriter: w, r: r}, r)
	})
}

type fallbackWriter struct {
	http.ResponseWriter
	r        *http.Request
	replaced bool
}

func (f *fallbackWriter) WriteHeader(code int) {
	plain := strings.HasPrefix(f.Header().Get("Content-Type"), "text/plain")
	switch {
	case plain && code == http.StatusNotFound:
		f.replaced = true
		httpx.Problem(f.ResponseWriter, f.r, errorx.New(httpx.CodeNotFound, "no such endpoint"))
	case plain && code == http.StatusMethodNotAllowed:
		f.replaced = true
		httpx.Problem(f.ResponseWriter, f.r, errorx.New(httpx.CodeMethodNotAllowed, "method not allowed"))
	default:
		f.ResponseWriter.WriteHeader(code)
	}
}

func (f *fallbackWriter) Write(b []byte) (int, error) {
	if f.replaced {
		return len(b), nil // drop the router's plain-text body
	}
	return f.ResponseWriter.Write(b)
}
