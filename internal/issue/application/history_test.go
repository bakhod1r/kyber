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

// Every use case that changes an issue writes an issue.changed entry naming the actor.
func TestEveryChangeIsAttributed(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	_, _ = s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Other", Type: "task"})
	rec.reset()

	_, _ = s.Transition(ctx, dev, "KYB-1", "in_progress", "")
	is, _ = s.Get(ctx, dev, "KYB-1")
	title := "Login page"
	_, _ = s.Edit(ctx, dev, "KYB-1", application.EditIssue{Version: is.Version(), Title: &title, SprintSet: true, Sprint: "s-1"})
	_, _ = s.Rank(ctx, dev, "KYB-1", application.RankMove{After: "KYB-2"})
	_, _, _ = s.ReturnUnfinished(ctx, lead, "KYB", "s-1", "")

	var got []domain.IssueChanged
	for _, e := range rec.all() {
		if c, ok := e.(domain.IssueChanged); ok {
			got = append(got, c)
		}
	}
	if len(got) != 4 {
		t.Fatalf("changelogs = %+v", got)
	}
	expect := []struct {
		by    domain.UserID
		field string
	}{{dev, "status"}, {dev, "title"}, {dev, "rank"}, {lead, "sprint"}}
	for i, e := range expect {
		if got[i].By != e.by || got[i].Items[0].Field != e.field || got[i].Key.String() != "KYB-1" {
			t.Errorf("changelog %d = %+v, want by %s field %s", i, got[i], e.by, e.field)
		}
	}
	if got[1].Items[1].Field != "sprint" || got[1].Items[1].To != "s-1" {
		t.Errorf("one edit changing two fields is one entry with two items: %+v", got[1])
	}
}

// The projector turns outbox events into history lines; History shows them with names.
func TestHistoryProjectionAndQuery(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	_, _ = s.Transition(ctx, dev, "KYB-1", "in_progress", "")
	is, _ = s.Get(ctx, dev, "KYB-1")
	assignee := dev
	_, _ = s.Edit(ctx, dev, "KYB-1", application.EditIssue{Version: is.Version(), AssigneeSet: true, Assignee: assignee})
	_, _ = s.AddComment(ctx, dev, "KYB-1", "Looks good")
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	for n, e := range rec.all() {
		raw, _ := json.Marshal(e)
		msg := outbox.Message{ID: int64(n + 1), Name: e.EventName(), Payload: raw, At: at.Add(time.Duration(n) * time.Minute)}
		if err := s.OnHistoryEvent(ctx, msg); err != nil {
			t.Fatal(err)
		}
		_ = s.OnHistoryEvent(ctx, msg) // redelivery
	}
	_ = s.OnHistoryEvent(ctx, outbox.Message{ID: 99, Name: "issue.changed", Payload: []byte("garbage")})

	h, err := s.History(ctx, viewer, "KYB-1")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, e := range h {
		lines = append(lines, e.ByName+"|"+e.Field+"|"+e.FromName+"|"+e.ToName)
	}
	want := "Name of u-dev|created||Login,Name of u-dev|status|todo|in_progress,Name of u-dev|assignee||Name of u-dev,Name of u-dev|comment||Looks good"
	if strings.Join(lines, ",") != want {
		t.Fatalf("history:\n%s\nwant:\n%s", strings.Join(lines, ","), want)
	}
	if _, err := s.History(ctx, alien, "KYB-1"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("alien err = %v", err)
	}
}
