package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestEditSchedule(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})

	got, err := s.Edit(ctx, dev, "KYB-1", application.EditIssue{Version: is.Version(), StartDate: str("2026-10-13"), DueDate: str("2026-10-20")})
	if err != nil {
		t.Fatal(err)
	}
	if st, due := got.Schedule(); st != "2026-10-13" || due != "2026-10-20" {
		t.Fatalf("schedule = %q %q", st, due)
	}
	// Only the due date changes; the start is kept.
	got, err = s.Edit(ctx, dev, "KYB-1", application.EditIssue{Version: got.Version(), DueDate: str("")})
	if st, due := got.Schedule(); err != nil || st != "2026-10-13" || due != "" {
		t.Fatalf("schedule = %q %q %v", st, due, err)
	}
	for _, cmd := range []application.EditIssue{
		{Version: got.Version(), DueDate: str("2026-10-01")}, // before the kept start
		{Version: got.Version(), StartDate: str("someday")},
		{Version: got.Version(), DueDate: str("2026-13-01")},
	} {
		if _, err := s.Edit(ctx, dev, "KYB-1", cmd); !errors.Is(err, domain.ErrInvalidSchedule) {
			t.Errorf("%+v: %v", cmd, err)
		}
	}
}
