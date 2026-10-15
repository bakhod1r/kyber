package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

// failingClose is the sprint store whose save of a closed sprint fails (a lost database, a concurrent edit).
type failingClose struct{ domain.Repository }

func (f failingClose) Save(ctx context.Context, s *domain.Sprint, ev []domain.Event) error {
	if s.State() == domain.StateClosed {
		return errors.New("db down")
	}
	return f.Repository.Save(ctx, s, ev)
}

// QA-05: completing a sprint is all or nothing. When the closed sprint cannot be saved, no issue has moved;
// once saved, the carry-over is a fact in the sprint.completed event that Issue Tracking acts on.
func TestCompletingASprintIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	s, repo, _ := setupWith(func(r domain.Repository) domain.Repository { return failingClose{r} })
	a, _ := s.Create(ctx, dev, "KYB", "A", "")
	next, _ := s.Create(ctx, dev, "KYB", "B", "")
	_, _ = s.Start(ctx, dev, string(a.ID()), time.Time{})
	if _, err := s.Complete(ctx, dev, string(a.ID()), string(next.ID())); err == nil {
		t.Fatal("expected the save error")
	}
	for _, name := range repo.OutboxNames() {
		if name == "sprint.completed" {
			t.Fatal("sprint.completed written although the sprint was not saved")
		}
	}
	if stored, _ := repo.ByID(ctx, a.ID()); stored.State() != domain.StateActive {
		t.Fatal("sprint must stay active")
	}
}
