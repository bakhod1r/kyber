package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
)

func names(ws []app.Watcher) string {
	s := ""
	for _, w := range ws {
		s += w.ID + ","
	}
	return s
}

func TestWatchers(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	key := is.Key().String()

	ws, watching, err := s.Watchers(ctx, dev, key)
	if err != nil || !watching || names(ws) != dev+"," || ws[0].Name != "Name of "+dev {
		t.Fatalf("reporter auto-watches: %v %v %v", ws, watching, err)
	}
	// The assignee and commenters start watching.
	_, _ = s.Edit(ctx, dev, key, app.EditIssue{Version: is.Version(), AssigneeSet: true, Assignee: lead})
	if _, err := s.AddComment(ctx, lead, key, "on it"); err != nil {
		t.Fatal(err)
	}
	if err := s.Watch(ctx, viewer, key); err != nil {
		t.Fatal(err)
	}
	if ws, _, _ := s.Watchers(ctx, viewer, key); names(ws) != dev+","+lead+","+viewer+"," {
		t.Fatalf("watchers = %s", names(ws))
	}
	if err := s.Unwatch(ctx, dev, key); err != nil {
		t.Fatal(err)
	}
	if ids, _ := s.WatchersOf(ctx, key); len(ids) != 2 || ids[0] != lead {
		t.Fatalf("WatchersOf = %v", ids)
	}
	if _, watching, _ := s.Watchers(ctx, dev, key); watching {
		t.Fatal("still watching after unwatch")
	}
	for _, f := range []func() error{
		func() error { return s.Watch(ctx, "u-alien", key) },
		func() error { return s.Unwatch(ctx, "u-alien", key) },
		func() error { _, _, err := s.Watchers(ctx, "u-alien", key); return err },
	} {
		if err := f(); !errors.Is(err, app.ErrProjectNotFound) {
			t.Errorf("outsider: %v", err)
		}
	}
	if _, err := s.WatchersOf(ctx, "KYB-404"); err == nil {
		t.Fatal("missing issue")
	}
}
