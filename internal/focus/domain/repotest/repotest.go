// Package repotest is the contract every focus-session Repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/focus/domain"
)

const (
	WS  = "00000000-0000-4000-8000-000000000001"
	Ann = "80000000-0000-4000-8000-000000000001"
	Ben = "80000000-0000-4000-8000-000000000002"
)

func sid(n int) string { return "81000000-0000-4000-8000-00000000000" + string(rune('0'+n)) }

// Run needs a repository whose store already has the users Ann and Ben.
func Run(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	s1, _ := domain.Start(sid(1), Ann, WS, "KYB-1", t0, domain.DefaultFocus, nil)
	if err := r.Create(ctx, s1); err != nil {
		t.Fatal(err)
	}
	// One active session per user.
	dup, _ := domain.Start(sid(2), Ann, WS, "KYB-2", t0, domain.DefaultFocus, nil)
	if err := r.Create(ctx, dup); !errors.Is(err, domain.ErrAlreadyRunning) {
		t.Fatalf("second active err = %v", err)
	}
	other, _ := domain.Start(sid(3), Ben, WS, "KYB-1", t0.Add(time.Minute), domain.MaxFocus, nil)
	if err := r.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	got, err := r.Active(ctx, Ann)
	if err != nil || got.ID != sid(1) || got.Planned != domain.DefaultFocus || !got.StartedAt.Equal(t0) || got.State != domain.Running {
		t.Fatalf("active = %+v %v", got, err)
	}
	_ = got.Pause(t0.Add(10 * time.Minute))
	if err := r.Save(ctx, got); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.Active(ctx, Ann); p.State != domain.Paused || !p.PausedAt.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("paused = %+v", p)
	}
	_ = got.Resume(t0.Add(15*time.Minute), nil)
	_ = got.Stop(t0.Add(20 * time.Minute))
	if err := r.Save(ctx, got); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Active(ctx, Ann); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("after stop err = %v", err)
	}
	// Ann can start again once the previous session ended.
	if err := r.Create(ctx, dup); err != nil {
		t.Fatal(err)
	}
	hist, err := r.ForIssue(ctx, WS, "KYB-1")
	if err != nil || len(hist) != 2 || hist[0].ID != sid(3) || hist[1].State != domain.Interrupted ||
		hist[1].Focused(t0.Add(time.Hour)) != 15*time.Minute || hist[1].Reason != "stopped" {
		t.Fatalf("issue history = %+v %v", hist, err)
	}
	if h, _ := r.ForIssue(ctx, "90000000-0000-4000-8000-000000000009", "KYB-1"); len(h) != 0 {
		t.Fatalf("other workspace = %v", h)
	}
	mine, err := r.ForUser(ctx, Ann, t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil || len(mine) != 2 {
		t.Fatalf("ann's log = %v %v", mine, err)
	}
	if late, _ := r.ForUser(ctx, Ann, t0.Add(time.Hour), t0.Add(2*time.Hour)); len(late) != 0 {
		t.Fatalf("window = %v", late)
	}
	if _, err := r.Active(ctx, "bad"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("bad user err = %v", err)
	}
}
