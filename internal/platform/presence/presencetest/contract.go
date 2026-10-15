// Package presencetest is the contract every presence Store must pass.
package presencetest

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/presence"
)

// Workspaces and users are UUIDs so the PostgreSQL adapter can store them.
const (
	WS1 = "70000000-0000-4000-8000-000000000001"
	WS2 = "70000000-0000-4000-8000-000000000002"
	Ann = "71000000-0000-4000-8000-000000000001"
	Bob = "71000000-0000-4000-8000-000000000002"
	Cat = "71000000-0000-4000-8000-000000000003"
)

func Run(t *testing.T, s presence.Store) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	online := func(ws string, since time.Time, users ...string) []string {
		t.Helper()
		got, err := s.Online(ctx, ws, users, since)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// Replica A sees Ann and Bob, replica B sees Bob (twice is fine) and Cat in another workspace.
	if err := s.Beat(ctx, "a", []presence.Seat{{Workspace: WS1, User: Ann}, {Workspace: WS1, User: Bob}}, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Beat(ctx, "b", []presence.Seat{{Workspace: WS1, User: Bob}, {Workspace: WS1, User: Bob}, {Workspace: WS2, User: Cat}}, t0); err != nil {
		t.Fatal(err)
	}
	if got := online(WS1, t0.Add(-time.Second), Cat, Bob, Ann, Bob); !slices.Equal(got, []string{Bob, Ann}) {
		t.Fatalf("online in WS1 (request order, no duplicates, other workspace excluded) = %v", got)
	}
	// A beat replaces the replica's set: Ann left replica A, Bob is still on B.
	if err := s.Beat(ctx, "a", []presence.Seat{{Workspace: WS1, User: Bob}}, t0.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := online(WS1, t0.Add(-time.Second), Ann, Bob); !slices.Equal(got, []string{Bob}) {
		t.Fatalf("after Ann left = %v", got)
	}
	// Replica B crashed (no more beats): only A's fresh seat counts.
	if got := online(WS2, t0.Add(5*time.Second), Cat); len(got) != 0 {
		t.Fatalf("stale seat counted = %v", got)
	}
	n, err := s.Expire(ctx, t0.Add(5*time.Second))
	if err != nil || n != 2 {
		t.Fatalf("expire = %d %v (B's Bob and Cat)", n, err)
	}
	if got := online(WS1, t0.Add(-time.Hour), Bob); !slices.Equal(got, []string{Bob}) {
		t.Fatalf("A's fresh seat must survive expiry = %v", got)
	}
	if err := s.Beat(ctx, "a", nil, t0.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := online(WS1, t0.Add(-time.Hour), Bob); len(got) != 0 {
		t.Fatalf("an empty beat clears the replica = %v", got)
	}
	if got := online(WS1, t0, "not-a-uuid"); len(got) != 0 {
		t.Fatalf("unknown ids = %v", got)
	}
}
