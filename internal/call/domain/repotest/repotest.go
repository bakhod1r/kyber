// Package repotest is the contract every Call repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/call/domain"
)

const (
	WS    = "00000000-0000-4000-8000-000000000001"
	Conv  = "72000000-0000-4000-8000-000000000001"
	Other = "72000000-0000-4000-8000-000000000002"
	Lead  = "70000000-0000-4000-8000-000000000001"
	Dev   = "70000000-0000-4000-8000-000000000002"
)

func Run(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	c := domain.Start("74000000-0000-4000-8000-000000000001", WS, Conv, Lead, t0)
	if err := r.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, domain.Start("74000000-0000-4000-8000-000000000002", WS, Conv, Dev, t0)); !errors.Is(err, domain.ErrCallLive) {
		t.Fatalf("second live call = %v", err)
	}
	live, err := r.LiveIn(ctx, Conv)
	if err != nil || live.ID != c.ID || live.StartedBy != Lead || !live.StartedAt.Equal(t0) || live.Workspace != WS ||
		len(live.Participants) != 1 || live.Participants[0].Media != (domain.Media{Audio: true}) {
		t.Fatalf("live = %+v %v", live, err)
	}
	if _, err := r.LiveIn(ctx, Other); !errors.Is(err, domain.ErrCallNotFound) {
		t.Fatalf("none live = %v", err)
	}

	// Two writers load the same version: the second save loses.
	a, _ := r.ByID(ctx, c.ID)
	b, _ := r.ByID(ctx, c.ID)
	_ = a.Join(Dev, t0.Add(time.Minute))
	_ = a.SetMedia(Dev, domain.Media{Video: true, Screen: true})
	_ = a.Present(Dev, domain.PresentDoc, "doc-42")
	if err := r.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	_ = b.Leave(Lead, t0)
	if err := r.Save(ctx, b); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale save = %v", err)
	}
	got, _ := r.ByID(ctx, c.ID)
	if len(got.Participants) != 2 || got.Participants[1].User != Dev || got.Participants[1].Media != (domain.Media{Video: true, Screen: true}) ||
		!got.Participants[1].JoinedAt.Equal(t0.Add(time.Minute)) || got.Version != 1 ||
		got.Presentation == nil || *got.Presentation != (domain.Presentation{Kind: domain.PresentDoc, Ref: "doc-42", By: Dev}) {
		t.Fatalf("after save = %+v", got)
	}

	// Ending frees the conversation for a new call.
	_ = got.Leave(Lead, t0)
	_ = got.Leave(Dev, t0.Add(time.Hour))
	if err := r.Save(ctx, got); err != nil {
		t.Fatal(err)
	}
	if ended, _ := r.ByID(ctx, c.ID); ended.Live() || !ended.EndedAt.Equal(t0.Add(time.Hour)) || len(ended.Participants) != 0 {
		t.Fatalf("ended = %+v", ended)
	}
	if _, err := r.LiveIn(ctx, Conv); !errors.Is(err, domain.ErrCallNotFound) {
		t.Fatalf("live after end = %v", err)
	}
	if err := r.Create(ctx, domain.Start("74000000-0000-4000-8000-000000000003", WS, Conv, Dev, t0)); err != nil {
		t.Fatalf("new call after end = %v", err)
	}
	for _, id := range []domain.CallID{"74000000-0000-4000-8000-000000000009", "nope"} {
		if _, err := r.ByID(ctx, id); !errors.Is(err, domain.ErrCallNotFound) {
			t.Fatalf("missing %s = %v", id, err)
		}
	}
}
