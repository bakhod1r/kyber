package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/focus/adapter/memory"
	"github.com/bakhod1r/kyber/internal/focus/app"
	"github.com/bakhod1r/kyber/internal/focus/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

// fakeCalendar returns the user's meetings overlapping the asked window.
type fakeCalendar struct {
	meetings  map[string][]domain.Busy
	fail      bool
	failAfter int // fail every call after this many (0 = off)
	calls     int
}

func (f *fakeCalendar) Busy(_ context.Context, user string, from, to time.Time) ([]domain.Busy, error) {
	f.calls++
	if f.fail || (f.failAfter > 0 && f.calls > f.failAfter) {
		return nil, errors.New("calendar down")
	}
	var out []domain.Busy
	for _, b := range f.meetings[user] {
		if from.Before(b.End) && b.Start.Before(to) {
			out = append(out, b)
		}
	}
	return out, nil
}

type fakeIssues map[string]bool // "user/KEY" readable

func (f fakeIssues) CanView(_ context.Context, user, key string) error {
	if !f[user+"/"+key] {
		return app.ErrIssueNotFound
	}
	return nil
}

func setup() (*app.Service, *clock, *fakeCalendar, context.Context) {
	c := &clock{now: t0}
	cal := &fakeCalendar{meetings: map[string][]domain.Busy{}}
	n := 0
	s := app.NewService(app.Deps{Sessions: memory.NewRepository(), Calendar: cal,
		Issues: fakeIssues{"ann/KYB-1": true, "ben/KYB-1": true, "ann/KYB-2": true},
		NewID:  func() string { n++; return fmt.Sprintf("s-%d", n) }, Clock: c})
	return s, c, cal, tenant.With(context.Background(), "w-1")
}

func TestPomodoroFlow(t *testing.T) {
	s, c, _, ctx := setup()
	if _, err := s.Current(ctx, "ann"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("no session err = %v", err)
	}
	sess, err := s.Start(ctx, "ann", "KYB-1", 25)
	if err != nil || sess.Workspace != "w-1" || sess.State != domain.Running {
		t.Fatalf("start = %+v %v", sess, err)
	}
	if _, err := s.Start(ctx, "ann", "KYB-2", 25); !errors.Is(err, domain.ErrAlreadyRunning) {
		t.Fatalf("second start err = %v", err)
	}
	c.now = t0.Add(10 * time.Minute)
	if _, err := s.Pause(ctx, "ann"); err != nil {
		t.Fatal(err)
	}
	c.now = t0.Add(15 * time.Minute)
	if _, err := s.Resume(ctx, "ann"); err != nil {
		t.Fatal(err)
	}
	// Finishes on its own: 10 + 15 focused minutes → 09:30; read after that settles it.
	c.now = t0.Add(40 * time.Minute)
	if _, err := s.Current(ctx, "ann"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("finished session still current: %v", err)
	}
	log, total, err := s.IssueLog(ctx, "ann", "KYB-1")
	if err != nil || len(log) != 1 || log[0].State != domain.Completed || total != 25*time.Minute {
		t.Fatalf("log = %+v total=%s %v", log, total, err)
	}
	// A new session can start now, and stopping counts what was done.
	if _, err := s.Start(ctx, "ann", "KYB-1", 15); err != nil {
		t.Fatal(err)
	}
	c.now = c.now.Add(5 * time.Minute)
	if st, err := s.Stop(ctx, "ann"); err != nil || st.State != domain.Interrupted {
		t.Fatalf("stop = %+v %v", st, err)
	}
	if _, total, _ := s.IssueLog(ctx, "ann", "KYB-1"); total != 30*time.Minute {
		t.Fatalf("total = %s", total)
	}
	mine, err := s.MyLog(ctx, "ann", t0, t0.Add(24*time.Hour))
	if err != nil || len(mine) != 2 {
		t.Fatalf("my log = %v %v", mine, err)
	}
}

func TestMeetingsBlockFocus(t *testing.T) {
	s, c, cal, ctx := setup()
	cal.meetings["ann"] = []domain.Busy{{Title: "Planning", Start: t0.Add(20 * time.Minute), End: t0.Add(time.Hour)}}
	var conflict *domain.MeetingConflictError
	if _, err := s.Start(ctx, "ann", "KYB-1", 25); !errors.As(err, &conflict) || conflict.Title != "Planning" {
		t.Fatalf("start into meeting err = %v", err)
	}
	if _, err := s.Start(ctx, "ann", "KYB-1", 15); err != nil {
		t.Fatalf("a 15-minute session fits before the meeting: %v", err)
	}
	// A meeting booked meanwhile interrupts the running session when it starts.
	s2, c2, cal2, ctx2 := setup()
	_, _ = s2.Start(ctx2, "ben", "KYB-1", 25)
	cal2.meetings["ben"] = []domain.Busy{{Title: "Incident", Start: t0.Add(5 * time.Minute), End: t0.Add(20 * time.Minute)}}
	c2.now = t0.Add(6 * time.Minute)
	if _, err := s2.Current(ctx2, "ben"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("still running during meeting: %v", err)
	}
	log, _, _ := s2.IssueLog(ctx2, "ben", "KYB-1")
	if log[0].Reason != "meeting: Incident" {
		t.Fatalf("log = %+v", log[0])
	}
	_ = c
}

func TestGuards(t *testing.T) {
	s, _, cal, ctx := setup()
	if _, err := s.Start(ctx, "ann", "OPS-1", 25); !errors.Is(err, app.ErrIssueNotFound) {
		t.Fatalf("unreadable issue err = %v", err)
	}
	if _, _, err := s.IssueLog(ctx, "ben", "KYB-2"); !errors.Is(err, app.ErrIssueNotFound) {
		t.Fatalf("unreadable log err = %v", err)
	}
	if _, err := s.Start(ctx, "ann", "KYB-1", 5); !errors.Is(err, domain.ErrInvalidLength) {
		t.Fatalf("length err = %v", err)
	}
	for _, f := range []func() (*domain.Session, error){
		func() (*domain.Session, error) { return s.Pause(ctx, "ann") },
		func() (*domain.Session, error) { return s.Resume(ctx, "ann") },
		func() (*domain.Session, error) { return s.Stop(ctx, "ann") },
	} {
		if _, err := f(); !errors.Is(err, domain.ErrSessionNotFound) {
			t.Fatalf("no session err = %v", err)
		}
	}
	_, _ = s.Start(ctx, "ann", "KYB-1", 25)
	if _, err := s.Resume(ctx, "ann"); !errors.Is(err, domain.ErrNotPaused) {
		t.Fatalf("resume running err = %v", err)
	}
	cal.fail = true
	if _, err := s.Current(ctx, "ann"); err == nil {
		t.Fatal("calendar errors surface")
	}
	if _, err := s.Start(ctx, "ben", "KYB-1", 25); err == nil {
		t.Fatal("calendar errors surface on start")
	}
	// Without a tenant the default workspace applies.
	cal.fail = false
	sess, err := s.Start(context.Background(), "ben", "KYB-1", 25)
	if err != nil || sess.Workspace != tenant.Default {
		t.Fatalf("default = %+v %v", sess, err)
	}
}

type brokenSessions struct {
	domain.Repository
	op string
}

var errDB = errors.New("db down")

func (b *brokenSessions) Save(ctx context.Context, s *domain.Session) error {
	if b.op == "save" {
		return errDB
	}
	return b.Repository.Save(ctx, s)
}

func (b *brokenSessions) ForIssue(ctx context.Context, ws, issue string) ([]*domain.Session, error) {
	if b.op == "list" {
		return nil, errDB
	}
	return b.Repository.ForIssue(ctx, ws, issue)
}

func (b *brokenSessions) Active(ctx context.Context, user string) (*domain.Session, error) {
	if b.op == "active" {
		return nil, errDB
	}
	return b.Repository.Active(ctx, user)
}

func TestStorageAndCalendarErrors(t *testing.T) {
	c := &clock{now: t0}
	cal := &fakeCalendar{meetings: map[string][]domain.Busy{}}
	repo := &brokenSessions{Repository: memory.NewRepository()}
	s := app.NewService(app.Deps{Sessions: repo, Calendar: cal, Issues: fakeIssues{"ann/KYB-1": true},
		NewID: func() string { return "s-1" }, Clock: c})
	ctx := context.Background()
	_, _ = s.Start(ctx, "ann", "KYB-1", 25)
	_, _ = s.Pause(ctx, "ann")
	cal.failAfter, cal.calls = 1, 0 // settling succeeds, the resume window check fails
	if _, err := s.Resume(ctx, "ann"); err == nil {
		t.Fatal("calendar error on resume")
	}
	cal.failAfter = 0
	repo.op = "save"
	if _, err := s.Current(ctx, "ann"); !errors.Is(err, errDB) {
		t.Fatalf("save err = %v", err)
	}
	repo.op = "active"
	if _, err := s.Start(ctx, "ann", "KYB-1", 25); !errors.Is(err, errDB) {
		t.Fatalf("active err = %v", err)
	}
	repo.op = "list"
	if _, _, err := s.IssueLog(ctx, "ann", "KYB-1"); !errors.Is(err, errDB) {
		t.Fatalf("list err = %v", err)
	}
}
