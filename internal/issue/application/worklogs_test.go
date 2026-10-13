package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

func TestWorklogs(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	project := func() {
		for _, e := range rec.all() {
			payload, _ := json.Marshal(e)
			if err := s.OnWorklogEvent(ctx, outbox.Message{Name: e.EventName(), Payload: payload}); err != nil {
				t.Fatal(err)
			}
		}
		rec.reset()
	}
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Parser", Type: "task"})
	key := is.Key().String()
	orig, bad := "1d", "soon"
	if _, err := s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), OriginalEstimate: &bad}); !errors.Is(err, domain.ErrInvalidDuration) {
		t.Fatalf("bad estimate: %v", err)
	}
	if _, err := s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), RemainingEstimate: &bad}); !errors.Is(err, domain.ErrInvalidDuration) {
		t.Fatalf("bad remaining: %v", err)
	}
	is, err := s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), OriginalEstimate: &orig})
	if err != nil || is.TimeTracking().Original != 8*3600 {
		t.Fatalf("estimate: %v %+v", err, is.TimeTracking())
	}

	at := time.Date(2025, 12, 31, 9, 0, 0, 0, time.UTC) // before the fake clock's "now"
	w1, err := s.LogWork(ctx, dev, key, application.LogWork{Spent: "2h", Started: at, Comment: "lexer"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.LogWork(ctx, lead, key, application.LogWork{Spent: "30m"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LogWork(ctx, viewer, key, application.LogWork{Spent: "1h"}); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer logs work: %v", err)
	}
	if _, err := s.LogWork(ctx, dev, key, application.LogWork{Spent: "a while"}); !errors.Is(err, domain.ErrInvalidDuration) {
		t.Fatalf("bad duration: %v", err)
	}
	if _, err := s.LogWork(ctx, dev, key, application.LogWork{Spent: "1h", Comment: string(make([]rune, 2001))}); !errors.Is(err, domain.ErrInvalidWorklog) {
		t.Fatalf("long comment: %v", err)
	}
	project()
	list, err := s.Worklogs(ctx, viewer, key)
	if err != nil || len(list) != 2 || list[0].ID != w1.ID || list[0].AuthorName != "Name of "+dev || list[0].Comment != "lexer" {
		t.Fatalf("worklogs = %+v %v", list, err)
	}
	got, _ := s.Get(ctx, dev, key)
	if tt := got.TimeTracking(); tt.Spent != 2*3600+1800 || tt.Remaining != 8*3600-2*3600-1800 {
		t.Fatalf("time = %+v", tt)
	}

	// Only the author or an admin takes a worklog back.
	if err := s.DeleteWorklog(ctx, dev, string(list[1].ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member deletes another's worklog: %v", err)
	}
	if err := s.DeleteWorklog(ctx, dev, string(w1.ID)); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWorklog(ctx, lead, string(list[1].ID)); err != nil {
		t.Fatal(err)
	}
	project()
	if list, _ := s.Worklogs(ctx, dev, key); len(list) != 0 {
		t.Fatalf("after delete = %+v", list)
	}
	if got, _ := s.Get(ctx, dev, key); got.TimeTracking().Spent != 0 || got.TimeTracking().Remaining != 8*3600 {
		t.Fatalf("time after delete = %+v", got.TimeTracking())
	}
	if err := s.DeleteWorklog(ctx, dev, string(w1.ID)); !errors.Is(err, domain.ErrWorklogNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := s.Worklogs(ctx, "u-alien", key); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider: %v", err)
	}
	// Outsiders cannot learn a worklog exists; malformed events are skipped.
	w3, _ := s.LogWork(ctx, dev, key, application.LogWork{Spent: "1m"})
	project()
	if err := s.DeleteWorklog(ctx, "u-alien", string(w3.ID)); !errors.Is(err, domain.ErrWorklogNotFound) {
		t.Fatalf("outsider delete: %v", err)
	}
	if err := s.OnWorklogEvent(ctx, outbox.Message{Name: "issue.work_logged", Payload: []byte("garbage")}); err != nil {
		t.Fatal(err)
	}
}
