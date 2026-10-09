package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func order(t *testing.T, s *app.Service, q app.ListQuery) string {
	t.Helper()
	list, err := s.List(context.Background(), dev, "KYB", q)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, len(list))
	for i, is := range list {
		keys[i] = is.Key().String()
	}
	return strings.Join(keys, ",")
}

func seed(t *testing.T, n int) *app.Service {
	t.Helper()
	s, _ := setup()
	for range n {
		if _, err := s.Create(context.Background(), dev, app.CreateIssue{Project: "KYB", Title: "t", Type: "task"}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestNewIssuesGoToTheBottom(t *testing.T) {
	s := seed(t, 3)
	if got := order(t, s, app.ListQuery{}); got != "KYB-1,KYB-2,KYB-3" {
		t.Fatalf("order = %s", got)
	}
}

func TestRankMoves(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 4)
	steps := []struct {
		key  string
		move app.RankMove
		want string
	}{
		{"KYB-4", app.RankMove{Before: "KYB-1"}, "KYB-4,KYB-1,KYB-2,KYB-3"}, // to the top
		{"KYB-4", app.RankMove{After: "KYB-3"}, "KYB-1,KYB-2,KYB-3,KYB-4"},  // to the bottom
		{"KYB-1", app.RankMove{After: "KYB-2"}, "KYB-2,KYB-1,KYB-3,KYB-4"},  // swap with neighbour (skips itself)
		{"KYB-3", app.RankMove{Before: "KYB-1"}, "KYB-2,KYB-3,KYB-1,KYB-4"}, // up one slot (skips itself)
		{"KYB-2", app.RankMove{After: "KYB-2"}, "KYB-2,KYB-3,KYB-1,KYB-4"},  // self anchor = no-op
	}
	for _, st := range steps {
		if _, err := s.Rank(ctx, dev, st.key, st.move); err != nil {
			t.Fatalf("Rank(%s, %+v): %v", st.key, st.move, err)
		}
		if got := order(t, s, app.ListQuery{}); got != st.want {
			t.Fatalf("after %s %+v: %s, want %s", st.key, st.move, got, st.want)
		}
	}
}

func TestRankRejections(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 2)
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "OPS", Title: "o", Type: "task"})
	cases := []struct {
		actor, key string
		move       app.RankMove
		want       error
	}{
		{dev, "KYB-1", app.RankMove{}, app.ErrInvalidAnchor},
		{dev, "KYB-1", app.RankMove{After: "KYB-2", Before: "KYB-2"}, app.ErrInvalidAnchor},
		{dev, "KYB-1", app.RankMove{After: "OPS-1"}, app.ErrInvalidAnchor},
		{dev, "KYB-1", app.RankMove{After: "KYB-99"}, app.ErrInvalidAnchor},
		{dev, "KYB-1", app.RankMove{After: "garbage"}, app.ErrInvalidAnchor},
		{viewer, "KYB-1", app.RankMove{After: "KYB-2"}, app.ErrForbidden},
		{alien, "KYB-1", app.RankMove{After: "KYB-2"}, app.ErrProjectNotFound},
	}
	for _, c := range cases {
		if _, err := s.Rank(ctx, c.actor, c.key, c.move); !errors.Is(err, c.want) {
			t.Errorf("Rank(%s by %s, %+v) err = %v, want %v", c.key, c.actor, c.move, err, c.want)
		}
	}
}

func TestMoveIssuesIntoSprintAndFilter(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 3)
	is, _ := s.Get(ctx, dev, "KYB-2")
	if _, err := s.Edit(ctx, dev, "KYB-2", app.EditIssue{Version: is.Version(), SprintSet: true, Sprint: "s-1"}); err != nil {
		t.Fatal(err)
	}
	if got := order(t, s, app.ListQuery{Sprint: "s-1"}); got != "KYB-2" {
		t.Fatalf("sprint = %s", got)
	}
	if got := order(t, s, app.ListQuery{Sprint: app.Backlog}); got != "KYB-1,KYB-3" {
		t.Fatalf("backlog = %s", got)
	}
	is, _ = s.Get(ctx, dev, "KYB-1")
	if _, err := s.Edit(ctx, dev, "KYB-1", app.EditIssue{Version: is.Version(), SprintSet: true, Sprint: "s-closed"}); !errors.Is(err, app.ErrInvalidSprint) {
		t.Fatalf("closed/foreign sprint err = %v", err)
	}
}

func TestReturnUnfinished(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 3)
	for _, k := range []string{"KYB-1", "KYB-2", "KYB-3"} {
		is, _ := s.Get(ctx, dev, k)
		_, _ = s.Edit(ctx, dev, k, app.EditIssue{Version: is.Version(), SprintSet: true, Sprint: "s-1"})
	}
	_, _ = s.Transition(ctx, dev, "KYB-1", "in_progress")
	_, _ = s.Transition(ctx, dev, "KYB-1", "done")
	_, _ = s.Transition(ctx, dev, "KYB-2", "in_progress")

	completed, returned, err := s.ReturnUnfinished(ctx, "KYB", "s-1")
	if err != nil || completed != 1 || returned != 2 {
		t.Fatalf("ReturnUnfinished = %d, %d, %v", completed, returned, err)
	}
	if got := order(t, s, app.ListQuery{Sprint: "s-1"}); got != "KYB-1" {
		t.Fatalf("done issues stay in the closed sprint: %s", got)
	}
	if got := order(t, s, app.ListQuery{Sprint: app.Backlog}); got != "KYB-2,KYB-3" {
		t.Fatalf("backlog = %s", got)
	}
	if is, _ := s.Get(ctx, dev, "KYB-2"); is.Status() != domain.StatusInProgress {
		t.Fatal("returning to the backlog must keep the status")
	}
}
