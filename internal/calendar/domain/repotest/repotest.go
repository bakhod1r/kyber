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
	mk := func(id, title string, start time.Time, mins int, aud domain.Audience, attendees ...domain.UserID) *domain.Meeting {
		m, _ := domain.PlanMeeting(domain.MeetingID(id), WS, title, start, start.Add(time.Duration(mins)*time.Minute), Lead, attendees, aud)
		return m
	}
	standup := mk("71000000-0000-4000-8000-000000000001", "Standup", t0, 15, domain.Audience{}, Dev)
	standup.Conversation = "72000000-0000-4000-8000-000000000001" // S76: its huddle
	review := mk("71000000-0000-4000-8000-000000000002", "Review", t0.Add(3*time.Hour), 60, domain.Audience{Scope: domain.ScopeProject, Project: "KYB"}, Dev, Other)
	for _, m := range []*domain.Meeting{review, standup} {
		if err := r.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.ByID(ctx, standup.ID)
	if err != nil || got.Title != "Standup" || len(got.Attendees) != 2 || !got.Start.Equal(t0) || got.Workspace != WS || got.Organizer != Lead ||
		got.Audience.Scope != domain.ScopePeople || got.Reminded || len(got.Responses) != 0 || got.Conversation != standup.Conversation {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	if got, _ := r.ByID(ctx, review.ID); got.Audience != (domain.Audience{Scope: domain.ScopeProject, Project: "KYB"}) {
		t.Fatalf("audience = %+v", got.Audience)
	}
	day, err := r.ForUser(ctx, Lead, t0.Add(-time.Hour), t0.Add(24*time.Hour))
	if err != nil || len(day) != 2 || day[0].Title != "Standup" {
		t.Fatalf("lead day = %v %v", day, err)
	}
	if dev, _ := r.ForUser(ctx, Dev, t0.Add(-time.Hour), t0.Add(24*time.Hour)); len(dev) != 2 {
		t.Fatalf("dev day = %v", dev)
	}
	// Overlap is half-open: a window ending at 09:00 does not include the standup.
	if early, _ := r.ForUser(ctx, Dev, t0.Add(-time.Hour), t0); len(early) != 0 {
		t.Fatalf("early = %v", early)
	}
	if none, _ := r.ForUser(ctx, Other, t0, t0.Add(2*time.Hour)); len(none) != 0 {
		t.Fatalf("other = %v", none)
	}

	// Answers are stored per attendee and come back with the meeting.
	rv, _ := r.ByID(ctx, review.ID)
	_ = rv.Respond(Dev, domain.Going)
	_ = rv.Respond(Other, domain.Declined)
	for _, u := range []domain.UserID{Dev, Other} {
		if err := r.Respond(ctx, rv, u); err != nil {
			t.Fatal(err)
		}
	}
	_ = rv.Respond(Dev, domain.Maybe)
	_ = r.Respond(ctx, rv, Dev)
	if got, _ := r.ByID(ctx, review.ID); got.Responses[Dev] != domain.Maybe || got.Responses[Other] != domain.Declined || len(got.Responses) != 2 {
		t.Fatalf("responses = %v", got.Responses)
	}
	if day, _ := r.ForUser(ctx, Dev, t0, t0.Add(24*time.Hour)); day[1].Responses[Dev] != domain.Maybe {
		t.Fatalf("listed responses = %v", day[1].Responses)
	}

	// Reminders: due ones are listed until saved once.
	due, err := r.DueForReminder(ctx, t0.Add(2*time.Hour+50*time.Minute), t0.Add(3*time.Hour+5*time.Minute))
	if err != nil || len(due) != 1 || due[0].ID != review.ID {
		t.Fatalf("due = %v %v", due, err)
	}
	m := due[0]
	if !m.Remind(t0.Add(2*time.Hour + 50*time.Minute)) {
		t.Fatal("not due")
	}
	if err := r.SaveReminder(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveReminder(ctx, m); !errors.Is(err, domain.ErrAlreadyReminded) {
		t.Fatalf("second reminder: %v", err)
	}
	if due, _ := r.DueForReminder(ctx, t0, t0.Add(4*time.Hour)); len(due) != 1 || due[0].ID != standup.ID {
		t.Fatalf("after reminding = %v", due)
	}
	if got, _ := r.ByID(ctx, review.ID); !got.Reminded {
		t.Fatal("reminded flag not stored")
	}

	st, _ := r.ByID(ctx, standup.ID)
	st.Cancel(Lead)
	if err := r.Cancel(ctx, st); err != nil {
		t.Fatal(err)
	}
	gone := &domain.Meeting{ID: "not-a-uuid"}
	for _, err := range []error{r.Cancel(ctx, st), func() error { _, err := r.ByID(ctx, standup.ID); return err }(),
		func() error { _, err := r.ByID(ctx, "not-a-uuid"); return err }(), r.Cancel(ctx, gone), r.Respond(ctx, st, Dev)} {
		if !errors.Is(err, domain.ErrMeetingNotFound) {
			t.Fatalf("missing err = %v", err)
		}
	}
}
