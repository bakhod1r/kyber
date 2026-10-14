package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/memory"
)

type fakeHints struct{ projects []string }

func (h *fakeHints) PresenceChanged(_ context.Context, project string) {
	h.projects = append(h.projects, project)
}

func TestPresence(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	ops, _ := s.Create(ctx, dev, application.CreateIssue{Project: "OPS", Title: "Deploy", Type: "task"})
	key := is.Key().String()
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	hints := &fakeHints{}
	p := application.NewPresence(s, memory.NewPresenceRegistry(), hints, func() time.Time { return now })

	if vs, err := p.Viewers(ctx, dev, key); err != nil || len(vs) != 0 {
		t.Fatalf("nobody yet = %v %v", vs, err)
	}
	for _, who := range []string{dev, viewer, dev} {
		if err := p.Heartbeat(ctx, who, key); err != nil {
			t.Fatal(err)
		}
	}
	vs, err := p.Viewers(ctx, lead, key)
	if err != nil || len(vs) != 2 || vs[0].ID != dev || vs[1].ID != viewer || vs[0].Name != "Name of "+dev {
		t.Fatalf("viewers = %+v %v", vs, err)
	}
	if len(hints.projects) != 2 || hints.projects[0] != "KYB" {
		t.Fatalf("hints = %v (only on arrival)", hints.projects)
	}

	now = now.Add(30 * time.Second)
	_ = p.Heartbeat(ctx, viewer, key)
	now = now.Add(20 * time.Second) // dev last seen 50 s ago
	if vs, _ := p.Viewers(ctx, dev, key); len(vs) != 1 || vs[0].ID != viewer {
		t.Fatalf("after the TTL = %+v", vs)
	}
	if err := p.Heartbeat(ctx, dev, key); err != nil || len(hints.projects) != 3 {
		t.Fatalf("returning viewer is an arrival: %v %v", err, hints.projects)
	}

	if err := p.Heartbeat(ctx, viewer, ops.Key().String()); err == nil {
		t.Fatal("heartbeat on a hidden issue must fail")
	}
	if _, err := p.Viewers(ctx, viewer, ops.Key().String()); err == nil {
		t.Fatal("viewers of a hidden issue must fail")
	}
}
