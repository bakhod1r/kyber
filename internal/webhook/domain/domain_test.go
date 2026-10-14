package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func TestNewWebhook(t *testing.T) {
	w, err := domain.New("w1", "KYB", " https://ci.example.com/hook#x ", "s3cret", []string{"issue.created", " issue.created", "comment.added"}, "u1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if w.URL != "https://ci.example.com/hook" || !w.Active || !slices.Equal(w.Events, []string{"comment.added", "issue.created"}) {
		t.Fatalf("webhook = %+v", w)
	}
	if !w.Wants("issue.created") || w.Wants("sprint.started") {
		t.Fatal("wants by name")
	}
	if err := w.Reconfigure("http://localhost:9000/x", []string{"*"}, true); err != nil || !w.Wants("sprint.started") {
		t.Fatalf("all events: %v", err)
	}
	if err := w.Reconfigure("http://127.0.0.1:9000/x", []string{"*"}, false); err != nil || w.Wants("sprint.started") {
		t.Fatalf("inactive: %v", err)
	}
	if err := w.Reconfigure("ftp://x", []string{"*"}, true); !errors.Is(err, domain.ErrInvalidWebhook) || w.URL != "http://127.0.0.1:9000/x" {
		t.Fatalf("bad reconfigure kept? %v %s", err, w.URL)
	}
	if _, err := domain.New("w1", "KYB", "https://x.io", "", []string{"*"}, "u1", t0); !errors.Is(err, domain.ErrInvalidWebhook) {
		t.Fatalf("no secret: %v", err)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = "issue.e" + strings.Repeat("x", i+1)
	}
	for _, tc := range []struct {
		url    string
		events []string
	}{
		{"http://ci.example.com/hook", []string{"*"}},
		{"ftp://ci.example.com", []string{"*"}},
		{"https://user:pw@ci.example.com", []string{"*"}},
		{"/relative", []string{"*"}},
		{"https://%zz", []string{"*"}},
		{"https://" + strings.Repeat("a", 2050), []string{"*"}},
		{"https://ci.example.com", nil},
		{"https://ci.example.com", []string{"Issue.Created"}},
		{"https://ci.example.com", []string{"issue"}},
		{"https://ci.example.com", many},
	} {
		if _, err := domain.New("w", "KYB", tc.url, "s", tc.events, "u1", t0); !errors.Is(err, domain.ErrInvalidWebhook) {
			t.Errorf("%q %v accepted: %v", tc.url, tc.events, err)
		}
	}
}

func TestLocal(t *testing.T) {
	for host, want := range map[string]bool{"localhost": true, "LOCALHOST": true, "127.0.0.1": true, "::1": true, "10.0.0.1": false, "example.com": false} {
		if domain.Local(host) != want {
			t.Errorf("Local(%q) != %v", host, want)
		}
	}
}

func TestSignature(t *testing.T) {
	body := []byte(`{"a":1}`)
	sig := domain.Sign("key", body)
	// echo -n '{"a":1}' | openssl dgst -sha256 -hmac key
	if sig != "sha256=88a67f24bbcdaed0e6c997404bb79a743baf44c6bab2f4c27328e3009d22e342" {
		t.Fatalf("sig = %s", sig)
	}
	if !domain.Verify("key", body, sig) || domain.Verify("other", body, sig) || domain.Verify("", body, domain.Sign("", body)) {
		t.Fatal("verify")
	}
}

func TestDeliveryBackoff(t *testing.T) {
	d := domain.NewDelivery("d1", "ws", "w1", "issue.created", "1/w1", []byte("{}"), t0)
	if d.Status != domain.DeliveryPending || !d.NextAt.Equal(t0) {
		t.Fatalf("new = %+v", d)
	}
	d.Failed(500, strings.Repeat("é", 600), false, t0)
	if d.NextAt != t0.Add(30*time.Second) || d.LastStatus != 500 || len([]rune(d.LastError)) != 500 || d.Status != domain.DeliveryPending {
		t.Fatalf("first failure = %+v", d)
	}
	d.Failed(0, "timeout", false, t0)
	if d.NextAt != t0.Add(time.Minute) {
		t.Fatalf("second wait = %v", d.NextAt.Sub(t0))
	}
	for d.Attempts < domain.MaxAttempts-1 {
		d.Failed(503, "busy", false, t0)
	}
	if d.NextAt != t0.Add(30*time.Minute) || d.Status != domain.DeliveryPending {
		t.Fatalf("capped wait = %v", d.NextAt.Sub(t0))
	}
	d.Failed(503, "busy", false, t0)
	if d.Status != domain.DeliveryFailed {
		t.Fatalf("after max attempts = %+v", d)
	}
	p := domain.NewDelivery("d2", "ws", "w1", "e.x", "k", nil, t0)
	p.Failed(0, "refused", true, t0)
	if p.Status != domain.DeliveryFailed || p.Attempts != 1 {
		t.Fatalf("permanent = %+v", p)
	}
	p.Delivered(204, t0)
	if p.Status != domain.DeliveryDelivered || p.LastError != "" || p.LastStatus != 204 || !p.DeliveredAt.Equal(t0) {
		t.Fatalf("delivered = %+v", p)
	}
}

func TestIssueKeys(t *testing.T) {
	got := domain.IssueKeys("KYB-12 fix crash (also OPS2-7, KYB-12)", "feature/KYB-3-login", "kyb-4 lower, X-1 too short, KYB-0")
	if !slices.Equal(got, []string{"KYB-12", "OPS2-7", "KYB-3"}) {
		t.Fatalf("keys = %v", got)
	}
	var many []string
	for i := 1; i <= 30; i++ {
		many = append(many, "AB-"+strings.Repeat("1", i))
	}
	if n := len(domain.IssueKeys(strings.Join(many, " "))); n != 20 {
		t.Fatalf("capped at %d", n)
	}
}

func TestNewLink(t *testing.T) {
	l, err := domain.NewLink("KYB-1", domain.LinkCommit, "o/r", "abc", "https://github.com/o/r/commit/abc", "  KYB-1 fix\n\nbody", "", t0)
	if err != nil || l.Title != "KYB-1 fix" {
		t.Fatalf("link = %+v %v", l, err)
	}
	l, _ = domain.NewLink("KYB-1", domain.LinkPullRequest, "o/r", "7", "http://ghe.local/o/r/pull/7", strings.Repeat("x", 600), "open", t0)
	if r := []rune(l.Title); len(r) != 500 || r[499] != '…' {
		t.Fatalf("title = %d", len(r))
	}
	for _, bad := range []domain.Link{
		{Issue: "kyb-1", Kind: domain.LinkCommit, Repo: "o/r", Ref: "a", URL: "https://x"},
		{Issue: "KYB-1", Kind: "tag", Repo: "o/r", Ref: "a", URL: "https://x"},
		{Issue: "KYB-1", Kind: domain.LinkCommit, Repo: "", Ref: "a", URL: "https://x"},
		{Issue: "KYB-1", Kind: domain.LinkCommit, Repo: "o/r", Ref: "", URL: "https://x"},
		{Issue: "KYB-1", Kind: domain.LinkCommit, Repo: "o/r", Ref: "a", URL: "javascript:alert(1)"},
	} {
		if _, err := domain.NewLink(bad.Issue, bad.Kind, bad.Repo, bad.Ref, bad.URL, "t", "", t0); !errors.Is(err, domain.ErrInvalidLink) {
			t.Errorf("accepted %+v", bad)
		}
	}
}
