package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

func TestLinks(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	for _, title := range []string{"API", "UI", "Docs"} {
		_, _ = s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: title, Type: "task"})
	}
	_, _ = s.Create(ctx, dev, application.CreateIssue{Project: "OPS", Title: "Deploy", Type: "task"})
	rec.reset()

	l, err := s.Link(ctx, dev, "KYB-1", "blocks", "KYB-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Link(ctx, dev, "KYB-3", "relates", "OPS-1"); err != nil {
		t.Fatalf("cross-project link: %v", err)
	}
	cases := []struct {
		actor, from, typ, to string
		want                 error
	}{
		{dev, "KYB-1", "blocks", "KYB-2", domain.ErrLinkExists},
		{dev, "KYB-1", "causes", "KYB-2", domain.ErrInvalidLink},
		{dev, "KYB-1", "blocks", "KYB-1", domain.ErrInvalidLink},
		{dev, "KYB-1", "blocks", "KYB-99", domain.ErrIssueNotFound},
		{viewer, "KYB-1", "relates", "KYB-3", application.ErrForbidden},
		{viewer, "OPS-1", "relates", "KYB-3", application.ErrProjectNotFound},
	}
	for _, c := range cases {
		if _, err := s.Link(ctx, c.actor, c.from, c.typ, c.to); !errors.Is(err, c.want) {
			t.Errorf("%s %s %s %s: err = %v, want %v", c.actor, c.from, c.typ, c.to, err, c.want)
		}
	}

	views, err := s.Links(ctx, viewer, "KYB-2")
	if err != nil || len(views) != 1 || views[0].Phrase != "is blocked by" || views[0].Other.Key().String() != "KYB-1" || views[0].Other.Title() != "API" {
		t.Fatalf("links of KYB-2 = %+v, %v", views, err)
	}
	// The viewer cannot see OPS, so the cross-project link is hidden from them.
	if v, _ := s.Links(ctx, viewer, "KYB-3"); len(v) != 0 {
		t.Fatalf("hidden link leaked: %+v", v)
	}
	if v, _ := s.Links(ctx, dev, "KYB-3"); len(v) != 1 || v[0].Phrase != "relates to" {
		t.Fatalf("dev sees = %+v", v)
	}

	if err := s.Unlink(ctx, viewer, string(l.ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer unlink err = %v", err)
	}
	if err := s.Unlink(ctx, alien, string(l.ID)); !errors.Is(err, domain.ErrLinkNotFound) {
		t.Fatalf("alien unlink err = %v", err)
	}
	if err := s.Unlink(ctx, dev, string(l.ID)); err != nil {
		t.Fatal(err)
	}

	// Both issues' histories show the link and its removal, with who did it.
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	for n, e := range rec.all() {
		raw, _ := json.Marshal(e)
		_ = s.OnHistoryEvent(ctx, outbox.Message{ID: int64(1000 + n), Name: e.EventName(), Payload: raw, At: at.Add(time.Duration(n) * time.Second)})
	}
	for key, want := range map[string]string{
		"KYB-1": "link:>blocks KYB-2,link:blocks KYB-2>",
		"KYB-2": "link:>is blocked by KYB-1,link:is blocked by KYB-1>",
	} {
		h, _ := s.History(ctx, dev, key)
		var lines []string
		for _, e := range h {
			if e.Field == "link" {
				lines = append(lines, e.Field+":"+e.From+">"+e.To)
			}
			if e.By != dev {
				t.Fatalf("by = %q", e.By)
			}
		}
		if strings.Join(lines, ",") != want {
			t.Errorf("%s history = %s, want %s", key, strings.Join(lines, ","), want)
		}
	}
}
