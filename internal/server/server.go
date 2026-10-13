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

	accessapp "github.com/bakhod1r/kyber/internal/access/application"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	accessmemory "github.com/bakhod1r/kyber/internal/access/infrastructure/memory"
	accesspg "github.com/bakhod1r/kyber/internal/access/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/access/infrastructure/projectcompat"
	accesshttp "github.com/bakhod1r/kyber/internal/access/interfaces/httpapi"
	agileapp "github.com/bakhod1r/kyber/internal/agile/application"
	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	agileissueacl "github.com/bakhod1r/kyber/internal/agile/infrastructure/issueacl"
	agilememory "github.com/bakhod1r/kyber/internal/agile/infrastructure/memory"
	agilepg "github.com/bakhod1r/kyber/internal/agile/infrastructure/postgres"
	agileprojectacl "github.com/bakhod1r/kyber/internal/agile/infrastructure/projectacl"
	agilehttp "github.com/bakhod1r/kyber/internal/agile/interfaces/httpapi"
	auditapp "github.com/bakhod1r/kyber/internal/audit/application"
	auditdomain "github.com/bakhod1r/kyber/internal/audit/domain"
	auditacl "github.com/bakhod1r/kyber/internal/audit/infrastructure/acl"
	auditmemory "github.com/bakhod1r/kyber/internal/audit/infrastructure/memory"
	auditpg "github.com/bakhod1r/kyber/internal/audit/infrastructure/postgres"
	audithttp "github.com/bakhod1r/kyber/internal/audit/interfaces/httpapi"
	calendarapp "github.com/bakhod1r/kyber/internal/calendar/application"
	calendardomain "github.com/bakhod1r/kyber/internal/calendar/domain"
	calendaracl "github.com/bakhod1r/kyber/internal/calendar/infrastructure/acl"
	calendarmemory "github.com/bakhod1r/kyber/internal/calendar/infrastructure/memory"
	calendarpg "github.com/bakhod1r/kyber/internal/calendar/infrastructure/postgres"
	calendarhttp "github.com/bakhod1r/kyber/internal/calendar/interfaces/httpapi"
	focusapp "github.com/bakhod1r/kyber/internal/focus/application"
	focusdomain "github.com/bakhod1r/kyber/internal/focus/domain"
	focusacl "github.com/bakhod1r/kyber/internal/focus/infrastructure/acl"
	focusmemory "github.com/bakhod1r/kyber/internal/focus/infrastructure/memory"
	focuspg "github.com/bakhod1r/kyber/internal/focus/infrastructure/postgres"
	focushttp "github.com/bakhod1r/kyber/internal/focus/interfaces/httpapi"
	identityapp "github.com/bakhod1r/kyber/internal/identity/application"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/argon2"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/guardlimit"
	identityjobs "github.com/bakhod1r/kyber/internal/identity/infrastructure/jobs"
	identitymemory "github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
	identitypg "github.com/bakhod1r/kyber/internal/identity/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/telegram"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	importerapp "github.com/bakhod1r/kyber/internal/importer/application"
	importerdomain "github.com/bakhod1r/kyber/internal/importer/domain"
	importeracl "github.com/bakhod1r/kyber/internal/importer/infrastructure/acl"
	importermemory "github.com/bakhod1r/kyber/internal/importer/infrastructure/memory"
	importerpg "github.com/bakhod1r/kyber/internal/importer/infrastructure/postgres"
	importerhttp "github.com/bakhod1r/kyber/internal/importer/interfaces/httpapi"
	insightsapp "github.com/bakhod1r/kyber/internal/insights/application"
	insightsdomain "github.com/bakhod1r/kyber/internal/insights/domain"
	insightsacl "github.com/bakhod1r/kyber/internal/insights/infrastructure/acl"
	insightsmemory "github.com/bakhod1r/kyber/internal/insights/infrastructure/memory"
	insightspg "github.com/bakhod1r/kyber/internal/insights/infrastructure/postgres"
	insightshttp "github.com/bakhod1r/kyber/internal/insights/interfaces/httpapi"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	issueaccessacl "github.com/bakhod1r/kyber/internal/issue/infrastructure/accessacl"
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/agileacl"
	issuememory "github.com/bakhod1r/kyber/internal/issue/infrastructure/memory"
	issuepg "github.com/bakhod1r/kyber/internal/issue/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/projectacl"
	issuehttp "github.com/bakhod1r/kyber/internal/issue/interfaces/httpapi"
	notifyapp "github.com/bakhod1r/kyber/internal/notify/application"
	notifydomain "github.com/bakhod1r/kyber/internal/notify/domain"
	notifyacl "github.com/bakhod1r/kyber/internal/notify/infrastructure/acl"
	notifymemory "github.com/bakhod1r/kyber/internal/notify/infrastructure/memory"
	notifypg "github.com/bakhod1r/kyber/internal/notify/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/telegramsend"
	notifyhttp "github.com/bakhod1r/kyber/internal/notify/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/metrics"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	projectapp "github.com/bakhod1r/kyber/internal/project/application"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/project/infrastructure/identitydir"
	projectmemory "github.com/bakhod1r/kyber/internal/project/infrastructure/memory"
	projectpg "github.com/bakhod1r/kyber/internal/project/infrastructure/postgres"
	projecthttp "github.com/bakhod1r/kyber/internal/project/interfaces/httpapi"
	searchapp "github.com/bakhod1r/kyber/internal/search/application"
	searchdomain "github.com/bakhod1r/kyber/internal/search/domain"
	searchacl "github.com/bakhod1r/kyber/internal/search/infrastructure/acl"
	searchmemory "github.com/bakhod1r/kyber/internal/search/infrastructure/memory"
	searchpg "github.com/bakhod1r/kyber/internal/search/infrastructure/postgres"
	searchhttp "github.com/bakhod1r/kyber/internal/search/interfaces/httpapi"
	workspaceapp "github.com/bakhod1r/kyber/internal/workspace/application"
	workspacedomain "github.com/bakhod1r/kyber/internal/workspace/domain"
	workspacememory "github.com/bakhod1r/kyber/internal/workspace/infrastructure/memory"
	workspacepg "github.com/bakhod1r/kyber/internal/workspace/infrastructure/postgres"
	workspacehttp "github.com/bakhod1r/kyber/internal/workspace/interfaces/httpapi"
	"github.com/bakhod1r/kyber/web"
)

// deps are the adapters a server is assembled from.
type deps struct {
	projects      projectdomain.Repository
	issues        issuedomain.Repository
	comments      issuedomain.CommentRepository
	workflows     issuedomain.WorkflowRepository
	history       issuedomain.HistoryRepository
	links         issuedomain.LinkRepository
	watchers      issuedomain.WatcherRepository
	worklogs      issuedomain.WorklogRepository
	fields        issuedomain.CustomFieldRepository
	audit         auditdomain.Repository
	schemes       accessdomain.SchemeRepository
	filters       searchdomain.FilterRepository
	sprints       agiledomain.Repository
	boards        agiledomain.BoardRepository
	versions      agiledomain.VersionRepository
	users         identitydomain.Users
	external      identitydomain.ExternalIdentities
	social        identityhttp.Social
	otps          identitydomain.OTPChallenges
	telegramBot   *telegram.Bot
	mailer        notifyapp.Sender
	publicURL     string
	dispatchEvery time.Duration
	deliveries    notifydomain.DeliveryRepository
	prefs         notifydomain.PrefsRepository
	groups        notifydomain.GroupRepository
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

// WithMailer sends notification emails (see smtpmail); without it, no email is sent.
func WithMailer(m notifyapp.Sender) Option { return func(d *deps) { d.mailer = m } }

// WithPublicURL is the base of links in emails and Telegram messages.
func WithPublicURL(u string) Option { return func(d *deps) { d.publicURL = u } }

// WithDispatchEvery sets how often email/Telegram deliveries are sent (default 5 s).
func WithDispatchEvery(every time.Duration) Option { return func(d *deps) { d.dispatchEvery = every } }

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
	versionRepo := agilememory.NewVersionRepository()
	d := deps{
		projects: projectmemory.NewRepository(), issues: issueRepo, comments: issuememory.NewCommentRepository(issueRepo),
		workflows: issuememory.NewWorkflowRepository(), history: issuememory.NewHistoryRepository(), links: issuememory.NewLinkRepository(issueRepo), watchers: issuememory.NewWatcherRepository(), worklogs: issuememory.NewWorklogRepository(), fields: issuememory.NewCustomFieldRepository(),
		audit: auditmemory.NewRepository(), schemes: accessmemory.NewRepository(), filters: searchmemory.NewFilterRepository(),
		sprints: sprintRepo, boards: agilememory.NewBoardRepository(), versions: versionRepo, notes: notifymemory.NewRepository(),
		deliveries: notifymemory.NewDeliveryRepository(), prefs: notifymemory.NewPrefsRepository(), groups: notifymemory.NewGroupRepository(), activity: insightsmemory.NewRepository(),
		mappings: importermemory.NewRepository(), workspaces: workspacememory.NewRepository(),
		meetings: calendarmemory.NewRepository(), focusSessions: focusmemory.NewRepository(),
		outbox: outbox.NewMemoryStore(events(issueRepo.Outbox, issueRepo.OutboxMeta), events(sprintRepo.Outbox, sprintRepo.OutboxMeta), events(versionRepo.Outbox, versionRepo.OutboxMeta)),
		users:  ids, sessions: ids, external: ids, otps: ids, ready: func(context.Context) error { return nil },
	}
	for _, o := range opts {
		o(&d)
	}
	return build(log, d)
}

// events adapts a context's typed event log to the relay's generic one.
func events[E outbox.Event](log func() []E, times func() []outbox.Meta) outbox.Source {
	return func() ([]outbox.Event, []outbox.Meta) {
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
	go identityjobs.RunSessionPurger(ctx, svc, purgeEvery, func(n int, err error) {
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
		workflows: issuepg.NewWorkflowRepository(pool), history: issuepg.NewHistoryRepository(pool), links: issuepg.NewLinkRepository(pool), watchers: issuepg.NewWatcherRepository(pool), worklogs: issuepg.NewWorklogRepository(pool), fields: issuepg.NewCustomFieldRepository(pool),
		audit: auditpg.NewRepository(pool), schemes: accesspg.NewRepository(pool), filters: searchpg.NewFilterRepository(pool),
		sprints: agilepg.NewRepository(pool), boards: agilepg.NewBoardRepository(pool), versions: agilepg.NewVersionRepository(pool), notes: notifypg.NewRepository(pool),
		deliveries: notifypg.NewDeliveryRepository(pool), prefs: notifypg.NewPrefsRepository(pool), groups: notifypg.NewGroupRepository(pool), outbox: outbox.NewPostgresStore(pool),
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
	// Access context (ADR-0003): every permission decision goes through the project's scheme.
	access := accessapp.NewService(projects).WithSchemes(d.schemes)
	acl := projectacl.New(projectcompat.NewGate(projects, access, accessdomain.AssignableUser), users)
	// Issue Tracking and Agile depend on each other through ports; the sprint
	// ACL is bound once both services exist.
	sprintACL := &lateSprints{}
	issues := issueapp.NewService(issueapp.Deps{
		Issues: d.issues, Comments: d.comments, Keys: acl, Access: issueaccessacl.New(access), Directory: acl, Sprints: sprintACL,
		Workflow: issuedomain.DefaultWorkflow(), Workflows: d.workflows, History: d.history, Links: d.links, Watchers: d.watchers, Worklogs: d.worklogs, Fields: d.fields, SecurityLevels: []string{accessapp.LevelConfidential}, NewID: id.New, Now: time.Now,
	})
	agileACL := agileprojectacl.New(projectcompat.NewGate(projects, access, accessdomain.ManageSprints))
	agile := agileapp.NewService(agileapp.Deps{
		Sprints: d.sprints, Boards: d.boards, Versions: d.versions, Issues: agileissueacl.NewIssues(issues), Access: agileACL, Admins: agileACL,
		Workflows: agileissueacl.New(issues), NewID: id.New, Now: time.Now,
	})
	sprintACL.Sprints = agileacl.New(agile)
	notify := notifyapp.NewService(notifyapp.Deps{
		Repo: d.notes, Issues: notifyacl.NewIssues(issues), Members: notifyacl.NewMembers(issues), Workspaces: notifyacl.NewWorkspaces(projects),
		Users: notifyacl.NewUsers(d.users), NewID: id.New, Now: time.Now, Log: log,
		Deliveries: d.deliveries, Prefs: d.prefs, Contacts: notifyacl.NewContacts(d.users, d.external),
		Channels: d.channels(), PublicURL: d.publicURL,
		Groups: d.groups, Admins: notifyacl.NewAdmins(access), BotName: d.social.TelegramBot,
		Lookups: notifyacl.NewLookups(issues, agile), Bot: d.botMessenger(),
	})
	search := searchapp.NewService(searchapp.Deps{Projects: searchacl.NewProjects(projects), Issues: searchacl.NewIssues(issues),
		Users: searchacl.NewUsers(d.users), Planning: searchacl.NewPlanning(agile), Filters: d.filters, NewID: id.New})
	insights := insightsapp.NewService(insightsapp.Deps{
		Repo: d.activity, Access: insightsacl.NewAccess(projectcompat.NewGate(projects, access, accessdomain.BrowseProjects)), Issues: insightsacl.NewIssues(issues),
		Users: insightsacl.NewUsers(d.users), Sprints: insightsacl.NewSprints(agile), Now: time.Now, Log: log,
	})
	importGate := projectcompat.NewGate(projects, access, accessdomain.AdministerProjects)
	importer := importerapp.NewService(importerapp.Deps{
		Access: importeracl.NewProject(importGate), Members: importeracl.NewProject(importGate),
		Issues: importeracl.NewIssues(issues, time.Now), Mappings: d.mappings, Source: importeracl.NewSource(issues, d.users),
	})
	calendar := calendarapp.NewService(d.meetings, calendaracl.NewMembers(workspaces, d.users), id.New, time.Now)
	focus := focusapp.NewService(focusapp.Deps{Sessions: d.focusSessions, Calendar: focusacl.NewCalendar(calendar),
		Issues: focusacl.NewIssues(issues, projectcompat.NewGate(projects, access, accessdomain.AssignableUser)), NewID: id.New, Clock: systemClock{}})
	relay := outbox.NewRelay(d.outbox, log)
	for _, name := range []string{"issue.created", "issue.transitioned", "issue.sprint_changed", "issue.estimated", "issue.imported"} {
		relay.Handle(name, insights.OnIssueEvent)
	}
	for _, name := range []string{"issue.created", "issue.changed", "comment.added", "issue.imported", "issue.linked", "issue.unlinked"} {
		relay.Handle(name, issues.OnHistoryEvent)
	}
	for _, name := range []string{"issue.created", "issue.changed", "sprint.started", "sprint.completed"} {
		relay.Handle(name, notify.OnProjectEvent)
	}
	relay.Handle("issue.assigned", notify.OnIssueAssigned)
	relay.Handle("comment.added", notify.OnCommentAdded)
	relay.Handle("issue.changed", notify.OnIssueChanged)
	for _, name := range []string{"issue.created", "issue.assigned", "comment.added"} {
		relay.Handle(name, issues.OnWatchTrigger)
	}
	for _, name := range []string{"issue.work_logged", "issue.worklog_removed"} {
		relay.Handle(name, issues.OnWorklogEvent)
	}
	for _, name := range []string{"version.released", "version.deleted"} {
		relay.Handle(name, issues.OnVersionEvent)
	}
	if d.ctx == nil {
		d.ctx = context.Background()
	}
	if d.relayEvery == 0 {
		d.relayEvery = time.Second
	}
	go relay.Run(d.ctx, d.relayEvery)
	go dispatch(d.ctx, log, notify, d.senders(), d.dispatchEvery)
	identityOpts := []identityapp.Option{identityapp.WithExternalIdentities(d.external)}
	if d.redis != nil {
		identityOpts = append(identityOpts, identityapp.WithLoginLimiter(guardlimit.New(d.redis, "kyber:")))
	}
	if d.telegramBot != nil {
		identityOpts = append(identityOpts, identityapp.WithTelegramOTP(d.otps, d.telegramBot))
		if c, ok := d.otps.(identitydomain.ConnectTokens); ok && d.social.TelegramBot != "" {
			identityOpts = append(identityOpts, identityapp.WithTelegramConnect(c, d.social.TelegramBot))
		}
		d.social.TelegramOTP = true
	}
	identity := identityapp.NewService(d.users, d.sessions, argon2.New(), systemClock{}, id.New, identityOpts...)
	if d.telegramBot != nil {
		d.telegramBot.HandleGroupStart(func(ctx context.Context, payload string, _ telegram.User, chat, title string) error {
			return notify.OnGroupStart(ctx, payload, chat, title)
		})
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
	accesshttp.New(access, log).Register(api)
	searchhttp.New(search, log).Register(api)
	agilehttp.New(agile, log).Register(api)
	notifyhttp.New(notify, log).Register(api)
	insightshttp.New(insights, log).Register(api)
	importerhttp.New(importer, log).Register(api)
	tenancy.Register(api)
	calendarhttp.New(calendar, log, time.Now).Register(api)
	focushttp.New(focus, log, time.Now).Register(api)
	audit := audithttp.New(auditapp.NewService(d.audit, auditacl.NewAdmins(projectcompat.NewGate(projects, access, accessdomain.AdministerProjects)), auditacl.NewNames(d.users), time.Now),
		auditacl.NewOwners(agile, d.boards, d.links), log)
	audit.Register(api)

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
	mux.Handle("/api/v1/", auth.RequireAuth(tenancy.RequireMember(audit.Record(problemFallback(api)))))
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

// channels lists the external notification channels this server can use.
func (d deps) channels() []notifydomain.Channel {
	var out []notifydomain.Channel
	if d.mailer != nil {
		out = append(out, notifydomain.ChannelEmail)
	}
	if d.telegramBot != nil {
		out = append(out, notifydomain.ChannelTelegram)
	}
	return out
}

func (d deps) senders() map[notifydomain.Channel]notifyapp.Sender {
	out := map[notifydomain.Channel]notifyapp.Sender{}
	if d.mailer != nil {
		out[notifydomain.ChannelEmail] = d.mailer
	}
	if d.telegramBot != nil {
		out[notifydomain.ChannelTelegram] = telegramsend.New(d.telegramBot)
	}
	return out
}

// dispatch sends due email/Telegram deliveries until ctx ends.
func dispatch(ctx context.Context, log *slog.Logger, notify *notifyapp.Service, senders map[notifydomain.Channel]notifyapp.Sender, every time.Duration) {
	if len(senders) == 0 {
		return
	}
	if every == 0 {
		every = 5 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for {
				n, err := notify.Dispatch(ctx, senders)
				if err != nil {
					log.ErrorContext(ctx, "notify: dispatch failed", "err", err)
				}
				if err != nil || n == 0 {
					break
				}
			}
		}
	}
}

// botMessenger is the bot as a notify.Messenger (nil when no bot is configured).
func (d deps) botMessenger() notifyapp.Messenger {
	if d.telegramBot == nil {
		return nil
	}
	return d.telegramBot
}
