package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/webhook/application"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
	"github.com/bakhod1r/kyber/internal/webhook/infrastructure/memory"
)

const (
	admin  = "u-admin"
	dev    = "u-dev"
	out    = "u-out"
	secret = "s3cret"
)

type projects struct{}

func (projects) AuthorizeAdmin(_ context.Context, actor, project string) error {
	switch {
	case project != "KYB" && project != "OPS", actor == out:
		return application.ErrProjectNotFound
	case actor != admin:
		return application.ErrForbidden
	}
	return nil
}

type issues struct{}

func (issues) Authorize(_ context.Context, actor, issue string) (string, error) {
	if actor == out || !strings.HasPrefix(strings.ToUpper(issue), "KYB-") {
		return "", application.ErrIssueNotFound
	}
	return strings.ToUpper(issue), nil
}

type sent struct {
	url     string
	headers map[string]string
	body    []byte
}

type sender struct {
	calls  []sent
	status int
	err    error
}

func (s *sender) Send(_ context.Context, url string, headers map[string]string, body []byte) (int, error) {
	s.calls = append(s.calls, sent{url, headers, body})
	return s.status, s.err
}

type fixture struct {
	svc   *application.Service
	store *memory.Store
	send  *sender
	now   *time.Time
	ctx   context.Context
}

func setup(t *testing.T, mod ...func(*application.Deps)) *fixture {
	t.Helper()
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.FixedZone("UZ", 5*3600))
	f := &fixture{store: memory.NewStore(), send: &sender{status: 200}, now: &now, ctx: tenant.With(context.Background(), tenant.Default)}
	n := 0
	d := application.Deps{Webhooks: f.store.Webhooks(), Deliveries: f.store.Deliveries(), Links: f.store.Links(),
		Projects: projects{}, Issues: issues{}, Sender: f.send,
		NewID: func() string { n++; return fmt.Sprintf("id-%d", n) }, NewSecret: func() string { return secret },
		Now: func() time.Time { return *f.now }}
	for _, m := range mod {
		m(&d)
	}
	f.svc = application.NewService(d)
	return f
}

func TestManageWebhooks(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	w, err := f.svc.Create(ctx, admin, "KYB", "https://ci.example.com/hook", []string{"issue.created"})
	if err != nil || w.Secret != secret || w.CreatedAt.Location() != time.UTC || w.CreatedBy != admin {
		t.Fatalf("create = %+v %v", w, err)
	}
	for _, tc := range []struct {
		actor, project, url string
		want                error
	}{
		{dev, "KYB", "https://x.io", application.ErrForbidden},
		{out, "KYB", "https://x.io", application.ErrProjectNotFound},
		{admin, "KYB", "http://x.io", domain.ErrInvalidWebhook},
	} {
		if _, err := f.svc.Create(ctx, tc.actor, tc.project, tc.url, []string{"*"}); !errors.Is(err, tc.want) {
			t.Errorf("create %+v: %v", tc, err)
		}
	}
	if list, err := f.svc.Webhooks(ctx, admin, "KYB"); err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if _, err := f.svc.Webhooks(ctx, dev, "KYB"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("dev lists: %v", err)
	}
	up, err := f.svc.Update(ctx, admin, "KYB", string(w.ID), "https://ci.example.com/v2", []string{"*"}, false)
	if err != nil || up.Active || up.URL != "https://ci.example.com/v2" || up.Secret != secret {
		t.Fatalf("update = %+v %v", up, err)
	}
	if _, err := f.svc.Update(ctx, admin, "KYB", string(w.ID), "ftp://x", []string{"*"}, true); !errors.Is(err, domain.ErrInvalidWebhook) {
		t.Fatalf("bad update: %v", err)
	}
	if _, err := f.svc.Update(ctx, admin, "OPS", string(w.ID), "https://x.io", []string{"*"}, true); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("other project's webhook: %v", err)
	}
	if _, err := f.svc.Update(ctx, dev, "KYB", string(w.ID), "https://x.io", []string{"*"}, true); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("dev updates: %v", err)
	}
	if _, err := f.svc.Deliveries(ctx, admin, "KYB", "missing"); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("missing deliveries: %v", err)
	}
	if list, err := f.svc.Deliveries(ctx, admin, "KYB", string(w.ID)); err != nil || len(list) != 0 {
		t.Fatalf("deliveries = %v %v", list, err)
	}
	if err := f.svc.Delete(ctx, admin, "OPS", string(w.ID)); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("delete via other project: %v", err)
	}
	if err := f.svc.Delete(ctx, admin, "KYB", string(w.ID)); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.svc.Webhooks(ctx, admin, "KYB"); len(list) != 0 {
		t.Fatalf("after delete = %v", list)
	}
}

func msg(id int64, name, payload string) outbox.Message {
	return outbox.Message{ID: id, Name: name, Payload: []byte(payload), At: time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC), Workspace: tenant.Default}
}

func TestDeliverEvents(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	all, _ := f.svc.Create(ctx, admin, "KYB", "https://a.example.com", []string{"*"})
	created, _ := f.svc.Create(ctx, admin, "KYB", "https://b.example.com", []string{"issue.created"})
	off, _ := f.svc.Create(ctx, admin, "KYB", "https://c.example.com", []string{"*"})
	if _, err := f.svc.Update(ctx, admin, "KYB", string(off.ID), off.URL, off.Events, false); err != nil {
		t.Fatal(err)
	}
	f.svc.Create(ctx, admin, "OPS", "https://d.example.com", []string{"*"})

	ev := msg(7, "issue.created", `{"id":"i-1","key":"KYB-1","title":"Secret plan","reporter":"u-1","type":"bug"}`)
	for range 2 { // redelivered by the relay: enqueued once
		if err := f.svc.OnEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.OnEvent(ctx, msg(8, "comment.added", `{"id":"c-1","issue_id":"i-1","issue_key":"KYB-1","body":"private","author":"u"}`)); err != nil {
		t.Fatal(err)
	}
	for _, m := range []outbox.Message{msg(9, "workspace.created", `{"name":"x"}`), msg(10, "x.y", `not json`), msg(11, "x.y", `{"key":"NOKEY"}`)} {
		if err := f.svc.OnEvent(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	n, err := f.svc.Dispatch(context.Background())
	if err != nil || n != 3 || len(f.send.calls) != 3 {
		t.Fatalf("dispatch = %d %v, calls %d", n, err, len(f.send.calls))
	}
	first := f.send.calls[0]
	if first.url != "https://a.example.com" || first.headers["X-Kyber-Event"] != "issue.created" || first.headers["Content-Type"] != "application/json" ||
		first.headers["X-Kyber-Signature"] != domain.Sign(secret, first.body) || first.headers["X-Kyber-Delivery"] == "" {
		t.Fatalf("first call = %+v", first)
	}
	var p map[string]any
	if err := json.Unmarshal(first.body, &p); err != nil {
		t.Fatal(err)
	}
	data := p["data"].(map[string]any)
	if p["id"] != first.headers["X-Kyber-Delivery"] || p["event"] != "issue.created" || p["project"] != "KYB" || p["occurred_at"] != "2026-10-14T03:00:00Z" ||
		data["key"] != "KYB-1" || data["id"] != "i-1" || data["project"] != "KYB" || len(data) != 3 {
		t.Fatalf("payload = %v", p)
	}
	if strings.Contains(string(first.body), "Secret plan") || strings.Contains(string(f.send.calls[2].body), "private") {
		t.Fatal("payload leaks content")
	}
	if f.send.calls[1].url != "https://b.example.com" || f.send.calls[2].headers["X-Kyber-Event"] != "comment.added" {
		t.Fatalf("calls = %+v", f.send.calls)
	}
	list, _ := f.svc.Deliveries(ctx, admin, "KYB", string(all.ID))
	if len(list) != 2 || list[0].Status != domain.DeliveryDelivered || list[0].LastStatus != 200 {
		t.Fatalf("deliveries = %+v", list)
	}
	if list, _ := f.svc.Deliveries(ctx, admin, "KYB", string(created.ID)); len(list) != 1 {
		t.Fatalf("filtered webhook got %d", len(list))
	}
	if n, _ := f.svc.Dispatch(context.Background()); n != 0 {
		t.Fatalf("delivered twice: %d", n)
	}
}

func TestRetries(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	w, _ := f.svc.Create(ctx, admin, "KYB", "https://a.example.com", []string{"*"})
	gone, _ := f.svc.Create(ctx, admin, "KYB", "https://gone.example.com", []string{"*"})
	f.svc.OnEvent(ctx, msg(1, "issue.created", `{"key":"KYB-1"}`))
	if err := f.svc.Delete(ctx, admin, "KYB", string(gone.ID)); err != nil {
		t.Fatal(err)
	}

	f.send.status = 503
	if n, err := f.svc.Dispatch(context.Background()); err != nil || n != 1 {
		t.Fatalf("dispatch = %d %v", n, err)
	}
	d, _ := f.svc.Deliveries(ctx, admin, "KYB", string(w.ID))
	if d[0].Status != domain.DeliveryPending || d[0].LastStatus != 503 || d[0].LastError != "HTTP 503" || d[0].Attempts != 1 {
		t.Fatalf("after 503 = %+v", d[0])
	}
	if n, _ := f.svc.Dispatch(context.Background()); n != 0 {
		t.Fatal("retried before the backoff")
	}
	*f.now = f.now.Add(time.Minute)
	f.send.status, f.send.err = 0, errors.New("connection refused")
	f.svc.Dispatch(context.Background())
	d, _ = f.svc.Deliveries(ctx, admin, "KYB", string(w.ID))
	if d[0].LastError != "connection refused" || d[0].Attempts != 2 {
		t.Fatalf("after error = %+v", d[0])
	}
	*f.now = f.now.Add(time.Hour)
	f.send.err = application.Permanent(errors.New("private address refused"))
	if !errors.Is(f.send.err, f.send.err.(interface{ Unwrap() error }).Unwrap()) {
		t.Fatal("permanent errors unwrap")
	}
	f.svc.Dispatch(context.Background())
	d, _ = f.svc.Deliveries(ctx, admin, "KYB", string(w.ID))
	if d[0].Status != domain.DeliveryFailed || d[0].Attempts != 3 {
		t.Fatalf("permanent = %+v", d[0])
	}

	// An inactive webhook's queued deliveries fail without being sent.
	f.svc.OnEvent(ctx, msg(2, "issue.created", `{"key":"KYB-2"}`))
	f.svc.Update(ctx, admin, "KYB", string(w.ID), w.URL, w.Events, false)
	calls := len(f.send.calls)
	f.svc.Dispatch(context.Background())
	d, _ = f.svc.Deliveries(ctx, admin, "KYB", string(w.ID))
	if len(f.send.calls) != calls || d[0].Status != domain.DeliveryFailed || d[0].LastError != "webhook is inactive" {
		t.Fatalf("inactive = %+v", d[0])
	}
}

var errBoom = errors.New("boom")

type failingWebhooks struct {
	domain.Repository
	list, byID, gone bool
}

func (f failingWebhooks) ForProject(ctx context.Context, p string) ([]*domain.Webhook, error) {
	if f.list {
		return nil, errBoom
	}
	return f.Repository.ForProject(ctx, p)
}

func (f failingWebhooks) ByID(ctx context.Context, id domain.ID) (*domain.Webhook, error) {
	if f.byID {
		return nil, errBoom
	}
	if f.gone {
		return nil, domain.ErrWebhookNotFound
	}
	return f.Repository.ByID(ctx, id)
}

type failingDeliveries struct {
	domain.DeliveryRepository
	enqueue, claim, save bool
}

func (f failingDeliveries) Enqueue(ctx context.Context, d domain.Delivery) error {
	if f.enqueue {
		return errBoom
	}
	return f.DeliveryRepository.Enqueue(ctx, d)
}

func (f failingDeliveries) Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	if f.claim {
		return nil, errBoom
	}
	return f.DeliveryRepository.Claim(ctx, now, lease, limit)
}

func (f failingDeliveries) Save(ctx context.Context, d domain.Delivery) error {
	if f.save {
		return errBoom
	}
	return f.DeliveryRepository.Save(ctx, d)
}

type failingLinks struct{ domain.LinkRepository }

func (failingLinks) Save(context.Context, domain.Link) error { return errBoom }

func TestStorageErrors(t *testing.T) {
	ev := msg(1, "issue.created", `{"key":"KYB-1"}`)
	seed := func(f *fixture) {
		if _, err := f.svc.Create(f.ctx, admin, "KYB", "https://a.example.com", []string{"*"}); err != nil {
			t.Fatal(err)
		}
	}
	f := setup(t)
	store := f.store
	wrap := func(w failingWebhooks, d failingDeliveries) *fixture {
		w.Repository, d.DeliveryRepository = store.Webhooks(), store.Deliveries()
		g := setup(t, func(deps *application.Deps) { deps.Webhooks, deps.Deliveries = w, d })
		return g
	}
	seed(f)
	if err := wrap(failingWebhooks{list: true}, failingDeliveries{}).svc.OnEvent(f.ctx, ev); !errors.Is(err, errBoom) {
		t.Fatalf("list error: %v", err)
	}
	if err := wrap(failingWebhooks{}, failingDeliveries{enqueue: true}).svc.OnEvent(f.ctx, ev); !errors.Is(err, errBoom) {
		t.Fatalf("enqueue error: %v", err)
	}
	if _, err := wrap(failingWebhooks{}, failingDeliveries{claim: true}).svc.Dispatch(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("claim error: %v", err)
	}
	if err := f.svc.OnEvent(f.ctx, ev); err != nil {
		t.Fatal(err)
	}
	if _, err := wrap(failingWebhooks{byID: true}, failingDeliveries{}).svc.Dispatch(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("load error: %v", err)
	}
	*f.now = f.now.Add(time.Hour) // past the lease
	g0 := wrap(failingWebhooks{gone: true}, failingDeliveries{})
	*g0.now = *f.now
	if n, err := g0.svc.Dispatch(context.Background()); err != nil || n != 1 || len(g0.send.calls) != 0 {
		t.Fatalf("deleted webhook: %d %v", n, err)
	}
	*f.now = f.now.Add(time.Hour)
	g := wrap(failingWebhooks{}, failingDeliveries{save: true})
	*g.now = *f.now
	if _, err := g.svc.Dispatch(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("save error: %v", err)
	}

	l := setup(t, func(d *application.Deps) { d.Links = failingLinks{} })
	if n, err := l.svc.RecordPush(l.ctx, application.Push{Repo: "o/r", Commits: []application.Commit{{SHA: "a", URL: "https://x", Message: "KYB-1"}}}); !errors.Is(err, errBoom) || n != 0 {
		t.Fatalf("link save error: %d %v", n, err)
	}
}

func TestDevelopment(t *testing.T) {
	f := setup(t)
	ctx := f.ctx
	n, err := f.svc.RecordPush(ctx, application.Push{Repo: "o/r", Commits: []application.Commit{
		{SHA: "abc", URL: "https://github.com/o/r/commit/abc", Message: "KYB-1 KYB-2: fix crash\n\nlong body"},
		{SHA: "def", URL: "https://github.com/o/r/commit/def", Message: "no key here"},
	}})
	if err != nil || n != 2 {
		t.Fatalf("push = %d %v", n, err)
	}
	*f.now = f.now.Add(time.Minute)
	n, err = f.svc.RecordPullRequest(ctx, application.PullRequest{Repo: "o/r", Number: 7, URL: "https://github.com/o/r/pull/7",
		Title: "Login page", Branch: "feature/KYB-1-login", State: "closed", Merged: true})
	if err != nil || n != 1 {
		t.Fatalf("pr = %d %v", n, err)
	}
	links, err := f.svc.Development(ctx, dev, "kyb-1")
	if err != nil || len(links) != 2 || links[0].Kind != domain.LinkPullRequest || links[0].State != "merged" || links[0].Ref != "7" ||
		links[1].Title != "KYB-1 KYB-2: fix crash" || links[1].UpdatedAt.Location() != time.UTC {
		t.Fatalf("links = %+v %v", links, err)
	}
	if _, err := f.svc.Development(ctx, out, "KYB-1"); !errors.Is(err, application.ErrIssueNotFound) {
		t.Fatalf("outsider: %v", err)
	}
	if n, err := f.svc.RecordPullRequest(ctx, application.PullRequest{Repo: "o/r", Number: 8, URL: "https://x", Title: "KYB-3", State: "open"}); err != nil || n != 1 {
		t.Fatalf("open pr = %d %v", n, err)
	}
	if n, err := f.svc.RecordPush(ctx, application.Push{Repo: "", Commits: []application.Commit{{SHA: "a", URL: "https://x", Message: "KYB-1"}}}); !errors.Is(err, domain.ErrInvalidLink) || n != 0 {
		t.Fatalf("invalid link = %d %v", n, err)
	}
}
