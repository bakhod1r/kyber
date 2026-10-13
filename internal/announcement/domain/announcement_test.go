package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func TestPublish(t *testing.T) {
	a, err := domain.Publish("a-1", "w-1", "KYB", " Release on Friday ", "Deploy at **16:00**.", "u-1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "Release on Friday" || a.Project != "KYB" || a.Author != "u-1" || !a.At.Equal(t0) {
		t.Fatalf("announcement = %+v", a)
	}
	evs := a.PullEvents()
	if len(evs) != 1 || evs[0] != (domain.Published{ID: "a-1", Workspace: "w-1", Project: "KYB", Title: "Release on Friday", Excerpt: "Deploy at **16:00**.", Author: "u-1"}) {
		t.Fatalf("events = %+v", evs)
	}
	if evs[0].EventName() != "announcement.published" || len(a.PullEvents()) != 0 {
		t.Fatal("events are pulled once")
	}
	if w, _ := domain.Publish("a-2", "w-1", "", "Holiday", "", "u-1", t0); !w.WorkspaceWide() {
		t.Fatal("no project = workspace-wide")
	}
	for _, tc := range []struct{ title, body string }{
		{" ", "x"},
		{strings.Repeat("x", 121), ""},
		{"Two\nlines", ""},
		{"Ok", strings.Repeat("x", 10_001)},
	} {
		if _, err := domain.Publish("a", "w", "", tc.title, tc.body, "u", t0); !errors.Is(err, domain.ErrInvalidAnnouncement) {
			t.Errorf("%q: %v", tc.title, err)
		}
	}
}

func TestExcerpt(t *testing.T) {
	a, _ := domain.Publish("a-1", "w-1", "", "Long", strings.Repeat("ab ", 200), "u-1", t0)
	ex := a.PullEvents()[0].(domain.Published).Excerpt
	if r := []rune(ex); len(r) != 280 || !strings.HasSuffix(ex, "…") {
		t.Fatalf("excerpt = %d runes", len(r))
	}
}
