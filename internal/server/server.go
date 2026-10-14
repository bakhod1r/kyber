// Package server wires bounded contexts into one HTTP handler (modular monolith).
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	attapp "github.com/bakhod1r/kyber/internal/attachment/application"
	attdomain "github.com/bakhod1r/kyber/internal/attachment/domain"
	attacl "github.com/bakhod1r/kyber/internal/attachment/infrastructure/acl"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobfs"
	attmemory "github.com/bakhod1r/kyber/internal/attachment/infrastructure/memory"
	attpg "github.com/bakhod1r/kyber/internal/attachment/infrastructure/postgres"
	atthttp "github.com/bakhod1r/kyber/internal/attachment/interfaces/httpapi"
	whapp "github.com/bakhod1r/kyber/internal/webhook/application"
	whdomain "github.com/bakhod1r/kyber/internal/webhook/domain"
	whacl "github.com/bakhod1r/kyber/internal/webhook/infrastructure/acl"
	"github.com/bakhod1r/kyber/internal/webhook/infrastructure/httpsend"
	whmemory "github.com/bakhod1r/kyber/internal/webhook/infrastructure/memory"
	whpg "github.com/bakhod1r/kyber/internal/webhook/infrastructure/postgres"
	whhttp "github.com/bakhod1r/kyber/internal/webhook/interfaces/httpapi"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/bakhod1r/errorx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	guard "github.com/bakhod1r/guard/ratelimit"
	accessapp "github.com/bakhod1r/kyber/internal/access/application"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	accessmemory "github.com/bakhod1r/kyber/internal/access/infrastructure/memory"
	accesspg "github.com/bakhod1r/kyber/internal/access/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/access/infrastructure/projectcompat"
	accesshttp "github.com/bakhod1r/kyber/internal/access/interfaces/httpapi"
	adminapp "github.com/bakhod1r/kyber/internal/admin/application"
	adminacl "github.com/bakhod1r/kyber/internal/admin/infrastructure/acl"
	adminhttp "github.com/bakhod1r/kyber/internal/admin/interfaces/httpapi"
	agileapp "github.com/bakhod1r/kyber/internal/agile/application"
	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	agileissueacl "github.com/bakhod1r/kyber/internal/agile/infrastructure/issueacl"
	agilememory "github.com/bakhod1r/kyber/internal/agile/infrastructure/memory"
	agilepg "github.com/bakhod1r/kyber/internal/agile/infrastructure/postgres"
	agileprojectacl "github.com/bakhod1r/kyber/internal/agile/infrastructure/projectacl"
	agilehttp "github.com/bakhod1r/kyber/internal/agile/interfaces/httpapi"
	annapp "github.com/bakhod1r/kyber/internal/announcement/application"
	anndomain "github.com/bakhod1r/kyber/internal/announcement/domain"
	annacl "github.com/bakhod1r/kyber/internal/announcement/infrastructure/acl"
	annmemory "github.com/bakhod1r/kyber/internal/announcement/infrastructure/memory"
	annpg "github.com/bakhod1r/kyber/internal/announcement/infrastructure/postgres"
	annhttp "github.com/bakhod1r/kyber/internal/announcement/interfaces/httpapi"
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
	callapp "github.com/bakhod1r/kyber/internal/call/application"
	calldomain "github.com/bakhod1r/kyber/internal/call/domain"
	callacl "github.com/bakhod1r/kyber/internal/call/infrastructure/acl"
	"github.com/bakhod1r/kyber/internal/call/infrastructure/ice"
	callmemory "github.com/bakhod1r/kyber/internal/call/infrastructure/memory"
	callpg "github.com/bakhod1r/kyber/internal/call/infrastructure/postgres"
	callrealtime "github.com/bakhod1r/kyber/internal/call/infrastructure/realtime"
	callhttp "github.com/bakhod1r/kyber/internal/call/interfaces/httpapi"
	chatapp "github.com/bakhod1r/kyber/internal/chat/application"
	chatdomain "github.com/bakhod1r/kyber/internal/chat/domain"
	chatacl "github.com/bakhod1r/kyber/internal/chat/infrastructure/acl"
	chatmemory "github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
	chatpg "github.com/bakhod1r/kyber/internal/chat/infrastructure/postgres"
	chatrealtime "github.com/bakhod1r/kyber/internal/chat/infrastructure/realtime"
	chathttp "github.com/bakhod1r/kyber/internal/chat/interfaces/httpapi"
	docsapp "github.com/bakhod1r/kyber/internal/docs/application"
	docsdomain "github.com/bakhod1r/kyber/internal/docs/domain"
	docsacl "github.com/bakhod1r/kyber/internal/docs/infrastructure/acl"
	docsmemory "github.com/bakhod1r/kyber/internal/docs/infrastructure/memory"
	docspg "github.com/bakhod1r/kyber/internal/docs/infrastructure/postgres"
	docshttp "github.com/bakhod1r/kyber/internal/docs/interfaces/httpapi"
	focusapp "github.com/bakhod1r/kyber/internal/focus/application"
	focusdomain "github.com/bakhod1r/kyber/internal/focus/domain"
	focusacl "github.com/bakhod1r/kyber/internal/focus/infrastructure/acl"
	focusmemory "github.com/bakhod1r/kyber/internal/focus/infrastructure/memory"
	focuspg "github.com/bakhod1r/kyber/internal/focus/infrastructure/postgres"
	focushttp "github.com/bakhod1r/kyber/internal/focus/interfaces/httpapi"
	formapp "github.com/bakhod1r/kyber/internal/form/application"
	formdomain "github.com/bakhod1r/kyber/internal/form/domain"
	formacl "github.com/bakhod1r/kyber/internal/form/infrastructure/acl"
	formmemory "github.com/bakhod1r/kyber/internal/form/infrastructure/memory"
	formpg "github.com/bakhod1r/kyber/internal/form/infrastructure/postgres"
	formhttp "github.com/bakhod1r/kyber/internal/form/interfaces/httpapi"
	goalapp "github.com/bakhod1r/kyber/internal/goal/application"
	goaldomain "github.com/bakhod1r/kyber/internal/goal/domain"
	goalacl "github.com/bakhod1r/kyber/internal/goal/infrastructure/acl"
	goalmemory "github.com/bakhod1r/kyber/internal/goal/infrastructure/memory"
	goalpg "github.com/bakhod1r/kyber/internal/goal/infrastructure/postgres"
	goalhttp "github.com/bakhod1r/kyber/internal/goal/interfaces/httpapi"
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
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/livehints"
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
	"github.com/bakhod1r/kyber/internal/platform/blob"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/live"
	"github.com/bakhod1r/kyber/internal/platform/metrics"
	"github.com/bakhod1r/kyber/internal/platform/otel"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/ratelimit"
	"github.com/bakhod1r/kyber/internal/platform/realtime"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
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
	wbapp "github.com/bakhod1r/kyber/internal/whiteboard/application"
	wbdomain "github.com/bakhod1r/kyber/internal/whiteboard/domain"
	wbacl "github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/acl"
	wbimages "github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/images"
	wbmemory "github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/memory"
	wbpg "github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/postgres"
	wbrealtime "github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/realtime"
	wbhttp "github.com/bakhod1r/kyber/internal/whiteboard/interfaces/httpapi"
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
	reactions     issuedomain.ReactionRepository
	worklogs      issuedomain.WorklogRepository
	fields        issuedomain.CustomFieldRepository
	audit         auditdomain.Repository
	schemes       accessdomain.SchemeRepository
	filters       searchdomain.FilterRepository
	goals         goaldomain.Repository
	pages         docsdomain.Repository
	docComments   docsdomain.Comments
	attachments   attdomain.Repository
	projectHooks  whdomain.Repository
	whDeliveries  whdomain.DeliveryRepository
	devLinks      whdomain.LinkRepository
	webhookOpen   bool          // deliver to private/loopback addresses too (KYBER_WEBHOOK_ALLOW_PRIVATE)
	webhookEvery  time.Duration // how often due webhook deliveries are sent (default 2s)
	githubSecret  string
	blobs         attapp.Blobs
	forms         formdomain.Repository
	announcements anndomain.Repository
	siteAdmins    map[string]bool // lower-case emails administering the default workspace
	sprints       agiledomain.Repository
	boards        agiledomain.BoardRepository
	versions      agiledomain.VersionRepository
	users         identitydomain.Users
	external      identitydomain.ExternalIdentities
	tokens        identitydomain.PersonalTokens
	oauth         identitydomain.OAuthStore
	directory     identitydomain.Directory
	scimToken     string
	social        identityhttp.Social
	otps          identitydomain.OTPChallenges
	telegramBot   *telegram.Bot
	mailer        notifyapp.Sender
	publicURL     string
	mqttAddr      string
	dispatchEvery time.Duration
	deliveries    notifydomain.DeliveryRepository
	prefs         notifydomain.PrefsRepository
	groups        notifydomain.GroupRepository
	telegramWait  time.Duration
	sessions      identitydomain.Sessions
	ready         func(context.Context) error
	cookieSecure  bool
	redis         redis.UniversalClient // optional: shared login limiter (guard)
	apiRate       int                   // requests per user per minute (0 = off)
	uploadRate    int                   // uploads per user per minute (0 = off)
	notes         notifydomain.Repository
	activity      insightsdomain.Repository
	mappings      importerdomain.MappingRepository
	workspaces    workspacedomain.Repository
	meetings      calendardomain.Repository
	focusSessions focusdomain.Repository
	conversations chatdomain.Repository
	messages      chatdomain.Messages
	saved         chatdomain.SavedMessages
	statuses      chatdomain.Statuses
	scheduled     chatdomain.ScheduledMessages
	reminders     chatdomain.Reminders
	userGroups    chatdomain.Groups // chat @groups (S70); groups are Telegram groups
	chatPrefs     chatdomain.Prefs
	transcriber   chatapp.Transcriber
	summarizer    chatapp.Summarizer
	bookmarks     chatdomain.Bookmarks
	chatWorkflows chatdomain.Workflows
	webhooks      chatdomain.Webhooks
	bots          chatdomain.Bots
	sectionOrders chatdomain.SectionOrders
	calls         calldomain.Repository
	whiteboards   wbdomain.Repository
	wbComments    wbdomain.Comments
	chatStore     blob.Store // chat files on S3 (S82)
	chatBlobs     blob.Store // chat files; memory unless WithFilesDir
	baseDomain    string     // "" = single-tenant (ADR-0004)
	ice           ice.Config
	media         MediaServer
	outbox        outbox.Store
	livePool      *pgxpool.Pool   // set: hints cross replicas through NOTIFY
	ctx           context.Context // lifetime of background work (outbox relay)
	relayEvery    time.Duration
	reminderEvery time.Duration // how often due meeting reminders are sent (default 30s)
}

// WithContext bounds background work (the outbox relay) to ctx.
func WithContext(ctx context.Context) Option { return func(d *deps) { d.ctx = ctx } }

// WithReminderEvery sets how often due meeting reminders are looked for (default 30s).
func WithReminderEvery(every time.Duration) Option { return func(d *deps) { d.reminderEvery = every } }

// WithRelayEvery sets how often the outbox relay polls (default 1s).
// WithMQTTAddr also accepts plain MQTT clients on addr (e.g. ":1883"; ADR-0013).
func WithMQTTAddr(addr string) Option { return func(d *deps) { d.mqttAddr = addr } }

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
// WithAttachmentsDir stores uploaded files under dir (default: in memory).
func WithAttachmentsDir(dir string) Option { return func(d *deps) { d.blobs = blobfs.New(dir) } }

// WithAttachmentStore stores uploaded files in blobs (e.g. an S3 bucket).
func WithAttachmentStore(blobs attapp.Blobs) Option { return func(d *deps) { d.blobs = blobs } }

// WithWebhooks configures outgoing webhooks: allowPrivate lets deliveries reach loopback and
// private addresses (otherwise refused against SSRF); every is the dispatch interval (0 = 2s).
func WithWebhooks(allowPrivate bool, every time.Duration) Option {
	return func(d *deps) { d.webhookOpen, d.webhookEvery = allowPrivate, every }
}

// WithGitHubSecret enables POST /api/v1/integrations/github, verified with this shared secret.
// WithRateLimits caps each signed-in user's API requests and uploads per minute (0 = off);
// with WithRedis the budget is shared across replicas.
func WithRateLimits(api, upload int) Option {
	return func(d *deps) { d.apiRate, d.uploadRate = api, upload }
}

// WithSCIMToken enables SCIM 2.0 provisioning at /scim/v2 for the identity provider holding token.
func WithSCIMToken(token string) Option { return func(d *deps) { d.scimToken = token } }

func WithGitHubSecret(secret string) Option { return func(d *deps) { d.githubSecret = secret } }

func WithPublicURL(u string) Option { return func(d *deps) { d.publicURL = u } }

// WithDispatchEvery sets how often email/Telegram deliveries are sent (default 5 s).
func WithDispatchEvery(every time.Duration) Option { return func(d *deps) { d.dispatchEvery = every } }

// WithBaseDomain enables workspaces on subdomains of domain (ADR-0004).
// WithSiteAdmins names (comma-separated emails) the administrators of the default workspace.
func WithSiteAdmins(emails string) Option {
	return func(d *deps) {
		d.siteAdmins = map[string]bool{}
		for _, e := range strings.Split(emails, ",") {
			if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
				d.siteAdmins[e] = true
			}
		}
	}
}

// siteAdmin reports whether user is a configured administrator of the default workspace.
func (d *deps) siteAdmin(ctx context.Context, user string) (bool, error) {
	if len(d.siteAdmins) == 0 {
		return false, nil
	}
	u, err := d.users.ByID(ctx, identitydomain.UserID(user))
	if err != nil {
		return false, err
	}
	return d.siteAdmins[strings.ToLower(string(u.Email()))], nil
}

func WithBaseDomain(domain string) Option { return func(d *deps) { d.baseDomain = domain } }

// WithICE sets the STUN/TURN servers for calls; TURN gets short-lived credentials from secret
// (coturn use-auth-secret).
func WithICE(urls []string, turnSecret string) Option {
	return func(d *deps) { d.ice = ice.Config{URLs: urls, Secret: turnSecret} }
}

// MediaServer is an SFU for large calls (ADR-0017): it issues room tickets, and the web app connects to its
// origins.
type MediaServer interface {
	callapp.Rooms
	Origins() []string
}

// WithMediaServer makes new calls rooms on the media server (LiveKit) instead of a browser mesh.
func WithMediaServer(m MediaServer) Option { return func(d *deps) { d.media = m } }

// WithTranscriber transcribes huddle recordings into their threads (S77).
func WithTranscriber(t chatapp.Transcriber) Option { return func(d *deps) { d.transcriber = t } }

// WithSummarizer adds a short summary to those transcripts (S87).
func WithSummarizer(s chatapp.Summarizer) Option { return func(d *deps) { d.summarizer = s } }

// WithChatFiles keeps chat files and recordings in s (e.g. S3, S82); it wins over WithFilesDir.
func WithChatFiles(s blob.Store) Option { return func(d *deps) { d.chatStore = s } }

// WithFilesDir keeps chat files on disk under dir (default: in memory, lost on restart).
func WithFilesDir(dir string) Option {
	return func(d *deps) {
		if s, err := blob.NewDisk(dir); err == nil {
			d.chatBlobs = s
		}
	}
}

// Option configures optional infrastructure.
type Option func(*deps)

// WithRedis shares the login limiter across replicas via guard's Redis limiter.
func WithRedis(rdb redis.UniversalClient) Option { return func(d *deps) { d.redis = rdb } }

// NewInMemory builds the API on in-memory adapters (dev mode and tests).
func NewInMemory(log *slog.Logger, opts ...Option) http.Handler {
	ids := identitymemory.NewRepository()
	chatRepo := chatmemory.NewRepository()
	savedRepo := chatmemory.NewSaved(chatRepo)
	remRepo := chatmemory.NewReminders()
	wbCommentRepo := wbmemory.NewComments()
	pageRepo := docsmemory.NewRepository()
	docCommentRepo := docsmemory.NewComments(pageRepo)
	callRepo := callmemory.NewRepository()
	issueRepo := issuememory.NewRepository()
	sprintRepo := agilememory.NewRepository()
	versionRepo := agilememory.NewVersionRepository()
	annRepo := annmemory.NewRepository()
	meetingRepo := calendarmemory.NewRepository()
	hooks := whmemory.NewStore()
	d := deps{
		projectHooks: hooks.Webhooks(), whDeliveries: hooks.Deliveries(), devLinks: hooks.Links(),
		projects: projectmemory.NewRepository(), issues: issueRepo, comments: issuememory.NewCommentRepository(issueRepo),
		workflows: issuememory.NewWorkflowRepository(), history: issuememory.NewHistoryRepository(), links: issuememory.NewLinkRepository(issueRepo), watchers: issuememory.NewWatcherRepository(), reactions: issuememory.NewReactionRepository(), worklogs: issuememory.NewWorklogRepository(), fields: issuememory.NewCustomFieldRepository(),
		audit: auditmemory.NewRepository(), schemes: accessmemory.NewRepository(), filters: searchmemory.NewFilterRepository(), goals: goalmemory.NewRepository(), pages: pageRepo, docComments: docCommentRepo, attachments: attmemory.NewRepository(), forms: formmemory.NewRepository(), announcements: annRepo,
		sprints: sprintRepo, boards: agilememory.NewBoardRepository(), versions: versionRepo, notes: notifymemory.NewRepository(),
		deliveries: notifymemory.NewDeliveryRepository(), prefs: notifymemory.NewPrefsRepository(), groups: notifymemory.NewGroupRepository(), activity: insightsmemory.NewRepository(),
		mappings: importermemory.NewRepository(), workspaces: workspacememory.NewRepository(),
		meetings: meetingRepo, focusSessions: focusmemory.NewRepository(),
		conversations: chatRepo, messages: chatRepo.Messages(), saved: savedRepo, statuses: chatmemory.NewStatuses(), scheduled: chatmemory.NewScheduled(), reminders: remRepo, userGroups: chatmemory.NewGroups(), chatPrefs: chatmemory.NewPrefs(), bookmarks: chatmemory.NewBookmarks(), chatWorkflows: chatmemory.NewWorkflows(), webhooks: chatmemory.NewWebhooks(), bots: chatmemory.NewBots(), sectionOrders: chatmemory.NewSectionOrders(), calls: callRepo, whiteboards: wbmemory.NewRepository(), wbComments: wbCommentRepo,
		outbox: outbox.NewMemoryStore(events(issueRepo.Outbox, issueRepo.OutboxMeta), events(sprintRepo.Outbox, sprintRepo.OutboxMeta), events(versionRepo.Outbox, versionRepo.OutboxMeta), events(annRepo.Outbox, annRepo.OutboxMeta), events(meetingRepo.Outbox, meetingRepo.OutboxMeta), events(chatRepo.Outbox, chatRepo.OutboxMeta), events(savedRepo.Outbox, savedRepo.OutboxMeta), events(remRepo.Outbox, remRepo.OutboxMeta), events(wbCommentRepo.Outbox, wbCommentRepo.OutboxMeta), events(docCommentRepo.Outbox, docCommentRepo.OutboxMeta), events(callRepo.Outbox, callRepo.OutboxMeta)),
		users:  ids, sessions: ids, external: ids, otps: ids, tokens: ids, oauth: ids, directory: ids, ready: func(context.Context) error { return nil },
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
		projectHooks: whpg.NewWebhooks(pool), whDeliveries: whpg.NewDeliveries(pool), devLinks: whpg.NewLinks(pool),
		projects: projectpg.NewRepository(pool), issues: issuepg.NewRepository(pool), comments: issuepg.NewCommentRepository(pool),
		workflows: issuepg.NewWorkflowRepository(pool), history: issuepg.NewHistoryRepository(pool), links: issuepg.NewLinkRepository(pool), watchers: issuepg.NewWatcherRepository(pool), reactions: issuepg.NewReactionRepository(pool), worklogs: issuepg.NewWorklogRepository(pool), fields: issuepg.NewCustomFieldRepository(pool),
		audit: auditpg.NewRepository(pool), schemes: accesspg.NewRepository(pool), filters: searchpg.NewFilterRepository(pool), goals: goalpg.NewRepository(pool), pages: docspg.NewRepository(pool), docComments: docspg.NewComments(pool), attachments: attpg.NewRepository(pool), forms: formpg.NewRepository(pool), announcements: annpg.NewRepository(pool),
		sprints: agilepg.NewRepository(pool), boards: agilepg.NewBoardRepository(pool), versions: agilepg.NewVersionRepository(pool), notes: notifypg.NewRepository(pool),
		deliveries: notifypg.NewDeliveryRepository(pool), prefs: notifypg.NewPrefsRepository(pool), groups: notifypg.NewGroupRepository(pool), outbox: outbox.NewPostgresStore(pool), livePool: pool,
		activity: insightspg.NewRepository(pool), mappings: importerpg.NewRepository(pool), workspaces: workspacepg.NewRepository(pool),
		meetings: calendarpg.NewRepository(pool), focusSessions: focuspg.NewRepository(pool),
		conversations: chatpg.NewRepository(pool), messages: chatpg.NewMessages(pool), saved: chatpg.NewSaved(pool), statuses: chatpg.NewStatuses(pool), scheduled: chatpg.NewScheduled(pool), reminders: chatpg.NewReminders(pool), userGroups: chatpg.NewGroups(pool), chatPrefs: chatpg.NewPrefs(pool), bookmarks: chatpg.NewBookmarks(pool), chatWorkflows: chatpg.NewWorkflows(pool), webhooks: chatpg.NewWebhooks(pool), bots: chatpg.NewBots(pool), sectionOrders: chatpg.NewSectionOrders(pool), calls: callpg.NewRepository(pool), whiteboards: wbpg.NewRepository(pool), wbComments: wbpg.NewComments(pool),
		users: ids, sessions: ids, external: ids, otps: ids, tokens: ids, oauth: ids, directory: ids, ready: pool.Ping, cookieSecure: cookieSecure,
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
		Workflow: issuedomain.DefaultWorkflow(), Workflows: d.workflows, History: d.history, Links: d.links, Watchers: d.watchers, Reactions: d.reactions, Worklogs: d.worklogs, Fields: d.fields, SecurityLevels: []string{accessapp.LevelConfidential}, NewID: id.New, Now: time.Now,
	})
	agileACL := agileprojectacl.New(projectcompat.NewGate(projects, access, accessdomain.ManageSprints))
	agile := agileapp.NewService(agileapp.Deps{
		Sprints: d.sprints, Boards: d.boards, Versions: d.versions, Issues: agileissueacl.NewIssues(issues), Access: agileACL, Admins: agileACL,
		Workflows: agileissueacl.New(issues), NewID: id.New, Now: time.Now,
	})
	sprintACL.Sprints = agileacl.New(agile)
	notify := notifyapp.NewService(notifyapp.Deps{
		Repo: d.notes, Issues: notifyacl.NewIssues(issues), Members: notifyacl.NewMembers(issues), Workspaces: notifyacl.NewWorkspaces(projects),
		Users: notifyacl.NewUsers(d.users), Audience: notifyacl.NewAudience(projects, workspaces), NewID: id.New, Now: time.Now, Log: log,
		Deliveries: d.deliveries, Prefs: d.prefs, Contacts: notifyacl.NewContacts(d.users, d.external),
		Channels: d.channels(), PublicURL: d.publicURL,
		Groups: d.groups, Admins: notifyacl.NewAdmins(access), BotName: d.social.TelegramBot,
		Lookups: notifyacl.NewLookups(issues, agile), Bot: d.botMessenger(),
	})
	search := searchapp.NewService(searchapp.Deps{Projects: searchacl.NewProjects(projects), Issues: searchacl.NewIssues(issues),
		Users: searchacl.NewUsers(d.users), Planning: searchacl.NewPlanning(agile), Filters: d.filters, NewID: id.New})
	goals := goalapp.NewService(goalapp.Deps{Projects: goalacl.NewProjects(projects), Issues: goalacl.NewIssues(search), Goals: d.goals, NewID: id.New})
	pages := docsapp.NewService(docsapp.Deps{Projects: docsacl.NewProjects(projects), Pages: d.pages, Comments: d.docComments, NewID: id.New, Now: time.Now})
	forms := formapp.NewService(formapp.Deps{Projects: formacl.NewProjects(projects), Issues: formacl.NewIssues(issues), Forms: d.forms, NewID: id.New})
	insights := insightsapp.NewService(insightsapp.Deps{
		Repo: d.activity, Access: insightsacl.NewAccess(projectcompat.NewGate(projects, access, accessdomain.BrowseProjects)), Issues: insightsacl.NewIssues(issues),
		Users: insightsacl.NewUsers(d.users), Sprints: insightsacl.NewSprints(agile), Now: time.Now, Log: log,
	})
	importGate := projectcompat.NewGate(projects, access, accessdomain.AdministerProjects)
	importer := importerapp.NewService(importerapp.Deps{
		Access: importeracl.NewProject(importGate), Members: importeracl.NewProject(importGate),
		Issues: importeracl.NewIssues(issues, time.Now), Mappings: d.mappings, Source: importeracl.NewSource(issues, d.users),
	})
	teams := calendaracl.NewTeams(projects, workspaces, d.siteAdmin)
	calendar := calendarapp.NewService(d.meetings, calendaracl.NewMembers(workspaces, d.users), id.New, time.Now).WithTeams(teams, teams)
	focus := focusapp.NewService(focusapp.Deps{Sessions: d.focusSessions, Calendar: focusacl.NewCalendar(calendar),
		Issues: focusacl.NewIssues(issues, projectcompat.NewGate(projects, access, accessdomain.AssignableUser)), NewID: id.New, Clock: systemClock{}})
	rt := realtime.NewHub(1024)
	// What contexts publish reaches every replica's streams in PostgreSQL mode (NOTIFY), this node's otherwise.
	var publish interface {
		Publish(ws string, users []string, typ string, data any) error
	} = rt
	if d.livePool != nil {
		fanout := realtime.NewPostgres(d.livePool, rt, log)
		go fanout.Listen(d.liveCtx())
		publish = fanout
	}
	if d.chatStore != nil {
		d.chatBlobs = d.chatStore
	}
	if d.chatBlobs == nil {
		log.Warn("KYBER_FILES_DIR not set: chat files are kept in memory and lost on restart")
		d.chatBlobs = blob.NewMemory()
	}
	chat := chatapp.NewService(chatapp.Deps{Blobs: d.chatBlobs, Notifier: chatrealtime.New(publish), Conversations: d.conversations, Messages: d.messages, Saved: d.saved, Statuses: d.statuses, Scheduled: d.scheduled, Reminders: d.reminders, Groups: d.userGroups, Prefs: d.chatPrefs, Projects: chatacl.NewProjects(projects), Transcriber: d.transcriber, Summarizer: d.summarizer, Bookmarks: d.bookmarks, Workflows: d.chatWorkflows, Webhooks: d.webhooks, SectionOrders: d.sectionOrders, Directory: chatacl.NewMembers(workspaces, d.users),
		Members: chatacl.NewMembers(workspaces, d.users), NewID: id.New, Now: time.Now})
	chat.WithIssues(chatacl.NewIssues(issues))
	calendar.WithConversations(calendaracl.NewConversations(chat)) // meetings in a channel's huddle (S76)
	callEvents := callrealtime.New(publish, func(c *calldomain.Call) any { return callhttp.ToDTO(c) })
	calls := callapp.NewService(callapp.Deps{Calls: d.calls, Conversations: callacl.NewConversations(chat),
		Notifier: callEvents, Signals: callEvents, Rooms: d.media, NewID: id.New, Now: time.Now})
	notify.WithConversations(notifyacl.NewConversations(chat))
	notify.WithQuiet(notifyacl.NewQuiet(chat, focus))
	notify.WithGroupMentions(notifyacl.NewGroupMentions(chat))
	notify.WithChatPrefs(chat)
	relay := outbox.NewRelay(d.outbox, log)
	var identity *identityapp.Service // built below; the broker's Login runs only after startup
	hub, err := live.NewBroker(func(ctx context.Context, user, project string) bool {
		return projects.Authorize(ctx, projectdomain.UserID(user), project, projectdomain.PermRead) == nil
	}, func(ctx context.Context, workspace, token string) (string, error) {
		u, err := identity.Authenticate(ctx, token)
		if err != nil {
			return "", err
		}
		if workspace != tenant.Default {
			if ok, err := workspaces.IsMember(ctx, workspace, string(u.ID())); err != nil || !ok {
				return "", errors.Join(errors.New("not a member of the workspace"), err)
			}
		}
		return string(u.ID()), nil
	}, log)
	if err != nil {
		panic(fmt.Sprintf("mqtt broker: %v", err)) // only fails on programming errors (duplicate hook or listener ids)
	}
	rt.Forward(hub) // S88: chat, calls and whiteboards also reach each user's MQTT topic
	if d.mqttAddr != "" {
		if err := hub.ListenTCP(d.mqttAddr); err != nil {
			panic(fmt.Sprintf("mqtt listener %s: %v", d.mqttAddr, err))
		}
	}
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
	relay.Handle("announcement.published", notify.OnAnnouncement)
	relay.Handle("announcement.published", notify.OnProjectEvent)
	for _, name := range []string{"meeting.scheduled", "meeting.rescheduled", "meeting.cancelled", "meeting.reminder"} {
		relay.Handle(name, notify.OnMeetingEvent)
		relay.Handle(name, notify.OnProjectEvent)
	}
	relay.Handle("comment.added", notify.OnCommentAdded)
	relay.Handle("chat.message_posted", notify.OnChatMessage)
	relay.Handle("chat.reminder", notify.OnChatReminder)
	relay.Handle("doc.comment_added", notify.OnDocComment)         // @mentions in doc comments (S86)
	relay.Handle("whiteboard.comment_added", notify.OnChatMessage) // @mentions in board comments (S86), as in its conversation
	relay.Handle("chat.message_posted", chat.OnRecordingPosted)    // transcripts (S77) and their summaries (S87)
	for _, name := range []string{"issue.created", "issue.transitioned", "sprint.started", "sprint.completed"} {
		relay.Handle(name, chat.OnProjectEvent) // project channels (S73)
	}
	relay.Handle("call.started", chat.OnCallEvent)
	relay.Handle("call.ended", chat.OnCallEvent)
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
	var hints live.Broadcaster = hub
	if d.livePool != nil {
		pg := live.NewPostgres(d.livePool, hub, log)
		go pg.Listen(d.liveCtx())
		hints = pg
	}
	relay.HandleAll(live.OnEvent(hints, log))
	// Presence is ephemeral and kept per replica (ADR-0013); the hint reaches every replica.
	presence := issueapp.NewPresence(issues, issuememory.NewPresenceRegistry(), livehints.New(hints, log), time.Now)
	webhooks := whapp.NewService(whapp.Deps{Webhooks: d.projectHooks, Deliveries: d.whDeliveries, Links: d.devLinks,
		Projects: whacl.NewProjects(projectcompat.NewGate(projects, access, accessdomain.AdministerProjects)), Issues: whacl.NewIssues(issues),
		Sender: httpsend.New(d.webhookOpen, 10*time.Second), NewID: id.New, NewSecret: newSecret, Now: time.Now})
	relay.HandleAll(webhooks.OnEvent)
	if d.ctx == nil {
		d.ctx = context.Background()
	}
	if d.relayEvery == 0 {
		d.relayEvery = time.Second
	}
	go relay.Run(d.ctx, d.relayEvery)
	go dispatch(d.ctx, log, notify, d.senders(), d.dispatchEvery)
	go remindMeetings(d.ctx, log, calendar.RemindDue, d.reminderEvery)
	go remindMeetings(d.ctx, log, chat.RemindDue, d.reminderEvery)    // saved messages (S65), /remind (S67)
	go remindMeetings(d.ctx, log, chat.SendDue, d.reminderEvery)      // scheduled messages (S67)
	go remindMeetings(d.ctx, log, chat.RunWorkflows, d.reminderEvery) // channel workflows (S80)
	go dispatchWebhooks(d.ctx, log, webhooks, d.webhookEvery)
	identityOpts := []identityapp.Option{identityapp.WithExternalIdentities(d.external), identityapp.WithPersonalTokens(d.tokens), identityapp.WithOAuth(d.oauth), identityapp.WithDirectory(d.directory)}
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
	identity = identityapp.NewService(d.users, d.sessions, argon2.New(), systemClock{}, id.New, identityOpts...)
	if d.telegramBot != nil {
		d.telegramBot.HandleGroupStart(func(ctx context.Context, payload string, _ telegram.User, chat, title string) error {
			return notify.OnGroupStart(ctx, payload, chat, title)
		})
		go d.telegramBot.Poll(d.ctx, log, d.telegramWait, 5*time.Second,
			func(ctx context.Context, nonce string, from telegram.User, chat string) error {
				return identity.OnTelegramStart(ctx, nonce, strconv.FormatInt(from.ID, 10), chat, from.DisplayName())
			})
	}
	chat.WithBots(d.bots, chatacl.NewBotMaker(identity), chatacl.NewWorkspace(workspaces, d.siteAdmin)) // bot users (S84)
	auth := identityhttp.New(identity, log, d.cookieSecure).WithSocial(d.social).WithPublicURL(d.publicURL)
	m := metrics.NewHTTP()

	api := http.NewServeMux()
	auth.RegisterProtected(api)
	projecthttp.New(projects, log).Register(api)
	issuehttp.New(issues, log).Register(api)
	issuehttp.NewPresence(presence, log).Register(api)
	accesshttp.New(access, log).Register(api)
	searchhttp.New(search, log).Register(api)
	goalhttp.New(goals, log).Register(api)
	docshttp.New(pages, log).Register(api)
	if d.blobs == nil {
		d.blobs = attmemory.NewBlobs()
	}
	atthttp.New(attapp.NewService(attapp.Deps{Issues: attacl.NewIssues(issues), Attachments: d.attachments, Blobs: d.blobs,
		NewID: id.New, Now: time.Now}), log).Register(api)
	annhttp.New(annapp.NewService(annapp.Deps{Access: annacl.NewAccess(projects, workspaces, d.siteAdmin), Announcements: d.announcements,
		NewID: id.New, Now: time.Now}), log).Register(api)
	formhttp.New(forms, log).Register(api)
	agilehttp.New(agile, log).Register(api)
	notifyhttp.New(notify, log).Register(api)
	webhookHTTP := whhttp.New(webhooks, log, d.githubSecret)
	webhookHTTP.Register(api)
	insightshttp.New(insights, log).Register(api)
	importerhttp.New(importer, log).Register(api)
	tenancy.Register(api)
	if dead, ok := d.outbox.(adminacl.Store); ok {
		adminhttp.New(adminapp.NewService(adminapp.Deps{
			Gate: adminacl.NewGate(workspaces, d.siteAdmin), Directory: adminacl.NewDirectory(workspaces, projects, d.users, d.siteAdmin),
			Projects: adminacl.NewProjects(projects), DeadLetters: adminacl.NewDeadLetters(dead),
		}), log).Register(api)
	}
	api.Handle("GET /api/v1/mqtt", hub.Handler())
	calendarhttp.New(calendar, log, time.Now).Register(api)
	focushttp.New(focus, log, time.Now).Register(api)
	chatAPI := chathttp.New(chat, log)
	chatAPI.Register(api)
	rt.Register(api, 25*time.Second) // board hints go over MQTT (ADR-0013)
	whiteboards := wbapp.NewService(wbapp.Deps{Boards: d.whiteboards, Comments: d.wbComments, Conversations: wbacl.NewConversations(chat),
		Images:   wbimages.New(d.chatBlobs), // S96: pictures live with chat files
		Notifier: wbrealtime.New(publish, func(e wbdomain.Element) any { return wbhttp.ToDTO(e) }), NewID: id.New, Now: time.Now})
	wbhttp.New(whiteboards, log).Register(api)
	m.Gauge("kyber_stream_connections", "Realtime streams open on this replica.", func() float64 { return float64(rt.Connections()) })
	m.Gauge("kyber_calls_active", "Live huddles in all workspaces.", func() float64 {
		n, err := calls.LiveCalls(context.Background())
		if err != nil {
			return -1
		}
		return float64(n)
	})
	callhttp.New(calls, log, func(user string, now time.Time) any { return d.ice.Servers(user, now) }).Register(api)
	audit := audithttp.New(auditapp.NewService(d.audit, auditacl.NewAdmins(projectcompat.NewGate(projects, access, accessdomain.AdministerProjects)), auditacl.NewNames(d.users), time.Now),
		auditacl.NewOwners(agile, d.boards, d.links), log)
	audit.Register(api)
	api.Handle("POST /api/v1/mcp", newMCP(projects, issues, search, agile))

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
	auth.RegisterSCIM(mux, d.scimToken)
	chatAPI.RegisterPublic(mux) // incoming webhooks (S81): the secret in the URL is the credential
	webhookHTTP.RegisterPublic(mux)
	mux.Handle("/api/v1/", auth.RequireAuth(d.rateLimit(log)(tenancy.RequireMember(audit.Record(problemFallback(otel.Routed(api)))))))
	var mediaOrigins []string
	if d.media != nil {
		mediaOrigins = d.media.Origins()
	}
	mux.Handle("/", web.Handler(web.Dist(), mediaOrigins...))
	return otel.Middleware(observe(log, m, recoverer(log, tenancy.Resolve(otel.Routed(mux)))))
}

// rateLimit limits each user's API requests: per process, or across replicas with Redis.
func (d *deps) rateLimit(log *slog.Logger) func(http.Handler) http.Handler {
	var l guard.Limiter = ratelimit.NewMemory(time.Now)
	if d.redis != nil {
		l = guard.NewRedis(d.redis, "kyber:rl:")
	}
	minute := func(n int) guard.Rule {
		if n <= 0 {
			return guard.Rule{}
		}
		return guard.Rule{Limit: n, Window: time.Minute}
	}
	return ratelimit.Middleware(l, ratelimit.Rules{API: minute(d.apiRate), Upload: minute(d.uploadRate)}, log)
}

// loggedPath is the request path as logged: incoming webhook URLs carry their secret, so it is cut (S81).
func loggedPath(p string) string {
	if strings.HasPrefix(p, "/api/v1/hooks/") {
		return "/api/v1/hooks/…"
	}
	return p
}

func observe(log *slog.Logger, m *metrics.HTTP, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		d := time.Since(start)
		m.Observe(r.Method, rec.status, d)
		log.InfoContext(r.Context(), "http", "method", r.Method, "path", loggedPath(r.URL.Path), "status", rec.status, "dur", d)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// Unwrap lets http.ResponseController reach the connection (flushing an event stream).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.ErrorContext(r.Context(), "panic", "value", fmt.Sprint(v), "path", loggedPath(r.URL.Path), "stack", string(debug.Stack()))
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

// Unwrap lets http.ResponseController reach the connection (flushing an event stream).
func (f *fallbackWriter) Unwrap() http.ResponseWriter { return f.ResponseWriter }

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
// remindMeetings sends due reminders (meetings, saved messages) with remindDue until ctx ends (once each, across
// replicas).
func remindMeetings(ctx context.Context, log *slog.Logger, remindDue func(context.Context) (int, error), every time.Duration) {
	if every == 0 {
		every = 30 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := remindDue(ctx); err != nil {
				log.WarnContext(ctx, "reminders: will retry", "err", err)
			}
		}
	}
}

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

// dispatchWebhooks sends due webhook deliveries until ctx ends.
func dispatchWebhooks(ctx context.Context, log *slog.Logger, webhooks *whapp.Service, every time.Duration) {
	if every == 0 {
		every = 2 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for {
				n, err := webhooks.Dispatch(ctx)
				if err != nil {
					log.ErrorContext(ctx, "webhooks: dispatch failed", "err", err)
				}
				if err != nil || n == 0 {
					break
				}
			}
		}
	}
}

// newSecret is a webhook's signing key: 32 random bytes, hex.
func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// botMessenger is the bot as a notify.Messenger (nil when no bot is configured).
func (d deps) botMessenger() notifyapp.Messenger {
	if d.telegramBot == nil {
		return nil
	}
	return d.telegramBot
}

// liveCtx bounds the replica's live listener like the relay (d.ctx may be set after construction).
func (d *deps) liveCtx() context.Context {
	if d.ctx == nil {
		return context.Background()
	}
	return d.ctx
}
