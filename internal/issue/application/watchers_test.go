package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/outbox"

	"github.com/bakhod1r/kyber/internal/issue/application"
)

func names(ws []application.Watcher) string {
	s := ""
	for _, w := range ws {
		s += w.ID + ","
	}
	return s
}

func TestWatchers(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	// relay feeds the recorded domain events to the watch trigger, as the outbox relay does.
	relay := func() {
		for _, e := range rec.all() {
			payload, _ := json.Marshal(e)
			if err := s.OnWatchTrigger(ctx, outbox.Message{Name: e.EventName(), Payload: payload}); err != nil {
				t.Fatal(err)
			}
		}
		rec.reset()
	}
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	key := is.Key().String()
	if ws, _, _ := s.Watchers(ctx, dev, key); len(ws) != 0 {
		t.Fatal("watching is a reaction to the event, not part of the command")
	}
	relay()

	ws, watching, err := s.Watchers(ctx, dev, key)
	if err != nil || !watching || names(ws) != dev+"," || ws[0].Name != "Name of "+dev {
		t.Fatalf("reporter auto-watches: %v %v %v", ws, watching, err)
	}
	// The assignee and commenters start watching.
	_, _ = s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), AssigneeSet: true, Assignee: lead})
	if _, err := s.AddComment(ctx, lead, key, "on it"); err != nil {
		t.Fatal(err)
	}
	relay()
	if err := s.OnWatchTrigger(ctx, outbox.Message{Name: "comment.added", Payload: []byte("garbage")}); err != nil {
		t.Fatal("malformed events are skipped")
	}
	if err := s.OnWatchTrigger(ctx, outbox.Message{Name: "issue.linked", Payload: []byte(`{}`)}); err != nil {
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
		if err := f(); !errors.Is(err, application.ErrProjectNotFound) {
			t.Errorf("outsider: %v", err)
		}
	}
	if _, err := s.WatchersOf(ctx, "KYB-404"); err == nil {
		t.Fatal("missing issue")
	}
}
