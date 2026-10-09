// Package repotest is the contract every meeting Repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
)

const (
	WS    = "00000000-0000-4000-8000-000000000001"
	Lead  = "70000000-0000-4000-8000-000000000001"
	Dev   = "70000000-0000-4000-8000-000000000002"
	Other = "70000000-0000-4000-8000-000000000003"
)

// Run needs a repository whose store already has the users Lead, Dev and Other.
func Run(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	mk := func(id, title string, start time.Time, mins int, attendees ...domain.UserID) *domain.Meeting {
		m, _ := domain.NewMeeting(domain.MeetingID(id), WS, title, start, start.Add(time.Duration(mins)*time.Minute), Lead, attendees)
		return m
	}
	standup := mk("71000000-0000-4000-8000-000000000001", "Standup", t0, 15, Dev)
	review := mk("71000000-0000-4000-8000-000000000002", "Review", t0.Add(3*time.Hour), 60)
	for _, m := range []*domain.Meeting{review, standup} {
		if err := r.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.ByID(ctx, standup.ID)
	if err != nil || got.Title != "Standup" || len(got.Attendees) != 2 || !got.Start.Equal(t0) || got.Workspace != WS || got.Organizer != Lead {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	day, err := r.ForUser(ctx, Lead, t0.Add(-time.Hour), t0.Add(24*time.Hour))
	if err != nil || len(day) != 2 || day[0].Title != "Standup" {
		t.Fatalf("lead day = %v %v", day, err)
	}
	if dev, _ := r.ForUser(ctx, Dev, t0.Add(-time.Hour), t0.Add(24*time.Hour)); len(dev) != 1 {
		t.Fatalf("dev day = %v", dev)
	}
	// Overlap is half-open: a window ending at 09:00 does not include the standup.
	if early, _ := r.ForUser(ctx, Dev, t0.Add(-time.Hour), t0); len(early) != 0 {
		t.Fatalf("early = %v", early)
	}
	if none, _ := r.ForUser(ctx, Other, t0, t0.Add(24*time.Hour)); len(none) != 0 {
		t.Fatalf("other = %v", none)
	}
	if err := r.Delete(ctx, standup.ID); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{r.Delete(ctx, standup.ID), func() error { _, err := r.ByID(ctx, standup.ID); return err }(),
		func() error { _, err := r.ByID(ctx, "not-a-uuid"); return err }(), r.Delete(ctx, "not-a-uuid")} {
		if !errors.Is(err, domain.ErrMeetingNotFound) {
			t.Fatalf("missing err = %v", err)
		}
	}
}
