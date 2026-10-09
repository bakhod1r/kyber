// Package repotest is the contract every agile domain.Repository adapter must satisfy.
package repotest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

// Factory returns an empty repository (projects KYB and OPS exist) and an outbox reader.
type Factory func(t *testing.T) (domain.Repository, func() []string)

func id(n int) domain.SprintID {
	return domain.SprintID("80000000-0000-4000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

func Run(t *testing.T, newRepo Factory) {
	ctx := context.Background()
	t0 := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	t.Run("save, load, update", func(t *testing.T) {
		repo, outbox := newRepo(t)
		s, _ := domain.NewSprint(id(1), "KYB", "Sprint 1", "Goal")
		if err := repo.Save(ctx, s, s.PullEvents()); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ByID(ctx, id(1))
		if err != nil || got.Name() != "Sprint 1" || got.Goal() != "Goal" || got.State() != domain.StatePlanned || got.Project() != "KYB" {
			t.Fatalf("ByID = %+v, %v", got, err)
		}
		_ = got.Start(t0, t0.Add(14*24*time.Hour))
		if err := repo.Save(ctx, got, got.PullEvents()); err != nil {
			t.Fatal(err)
		}
		again, _ := repo.ByID(ctx, id(1))
		if again.State() != domain.StateActive || !again.StartedAt().Equal(t0) || !again.EndsAt().Equal(t0.Add(14*24*time.Hour)) {
			t.Fatalf("after start: %q %v", again.State(), again.StartedAt())
		}
		if ev := strings.Join(outbox(), ","); ev != "sprint.created,sprint.started" {
			t.Fatalf("outbox = %s", ev)
		}
	})

	t.Run("stale copy is rejected (optimistic locking)", func(t *testing.T) {
		repo, outbox := newRepo(t)
		s, _ := domain.NewSprint(id(1), "KYB", "A", "")
		_ = repo.Save(ctx, s, s.PullEvents())
		x, _ := repo.ByID(ctx, id(1))
		y, _ := repo.ByID(ctx, id(1))
		_ = x.Start(t0, t0.Add(14*24*time.Hour))
		if err := repo.Save(ctx, x, x.PullEvents()); err != nil {
			t.Fatal(err)
		}
		_ = y.Start(t0.Add(time.Minute), t0.Add(15*24*time.Hour)) // y was loaded before x was saved
		if err := repo.Save(ctx, y, y.PullEvents()); !errors.Is(err, domain.ErrSprintConflict) {
			t.Fatalf("stale save err = %v, want ErrSprintConflict", err)
		}
		if got := strings.Join(outbox(), ","); got != "sprint.created,sprint.started" {
			t.Fatalf("outbox = %s (duplicate events)", got)
		}
		if again, _ := repo.ByID(ctx, id(1)); !again.StartedAt().Equal(t0) {
			t.Fatalf("stale save overwrote started_at: %v", again.StartedAt())
		}
		dup, _ := domain.NewSprint(id(1), "KYB", "dup", "")
		if err := repo.Save(ctx, dup, nil); !errors.Is(err, domain.ErrSprintConflict) {
			t.Fatalf("re-inserting an existing id err = %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, _ := newRepo(t)
		for _, missing := range []domain.SprintID{id(9), "not-a-uuid", ""} {
			if _, err := repo.ByID(ctx, missing); !errors.Is(err, domain.ErrSprintNotFound) {
				t.Fatalf("ByID(%q) err = %v", missing, err)
			}
		}
	})

	t.Run("one active sprint per project", func(t *testing.T) {
		repo, outbox := newRepo(t)
		a, _ := domain.NewSprint(id(1), "KYB", "A", "")
		b, _ := domain.NewSprint(id(2), "KYB", "B", "")
		o, _ := domain.NewSprint(id(3), "OPS", "O", "")
		for _, s := range []*domain.Sprint{a, b, o} {
			_ = repo.Save(ctx, s, nil)
		}
		_ = a.Start(t0, t0.Add(14*24*time.Hour))
		if err := repo.Save(ctx, a, a.PullEvents()); err != nil {
			t.Fatal(err)
		}
		_ = o.Start(t0, t0.Add(14*24*time.Hour))
		if err := repo.Save(ctx, o, nil); err != nil {
			t.Fatalf("other project may have its own active sprint: %v", err)
		}
		_ = b.Start(t0, t0.Add(14*24*time.Hour))
		before := len(outbox())
		if err := repo.Save(ctx, b, b.PullEvents()); !errors.Is(err, domain.ErrAnotherSprintLive) {
			t.Fatalf("second active err = %v", err)
		}
		if len(outbox()) != before {
			t.Fatal("rejected save must not write events")
		}
		if stored, _ := repo.ByID(ctx, id(2)); stored.State() != domain.StatePlanned {
			t.Fatal("rejected save must not persist")
		}
	})

	t.Run("list order: active, planned oldest first, closed newest first", func(t *testing.T) {
		repo, _ := newRepo(t)
		mk := func(n int, name string) *domain.Sprint {
			s, _ := domain.NewSprint(id(n), "KYB", name, "")
			_ = repo.Save(ctx, s, nil)
			time.Sleep(2 * time.Millisecond) // distinct creation times
			return s
		}
		c1, c2 := mk(1, "closed-old"), mk(2, "closed-new")
		p1, act, p2 := mk(3, "planned-old"), mk(4, "active"), mk(5, "planned-new")
		for i, s := range []*domain.Sprint{c1, c2} {
			_ = s.Start(t0, t0.Add(14*24*time.Hour))
			_ = repo.Save(ctx, s, nil)
			_ = s.Complete(t0.Add(time.Duration(i+1) * time.Hour))
			_ = repo.Save(ctx, s, nil)
		}
		_ = act.Start(t0, t0.Add(14*24*time.Hour))
		_ = repo.Save(ctx, act, nil)
		_, _ = p1, p2
		list, err := repo.ListByProject(ctx, "KYB")
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, len(list))
		for i, s := range list {
			names[i] = s.Name()
		}
		if got := strings.Join(names, ","); got != "active,planned-old,planned-new,closed-new,closed-old" {
			t.Fatalf("order = %s", got)
		}
		if other, _ := repo.ListByProject(ctx, "OPS"); len(other) != 0 {
			t.Fatalf("OPS sprints = %v", other)
		}
	})
}
