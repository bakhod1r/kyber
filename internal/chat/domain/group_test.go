package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestUserGroup(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	g, err := domain.NewGroup("g-1", "ws", " Backend ", "Backend team", []domain.UserID{"b", "a", "b"}, "a", t0)
	if err != nil || g.Handle != "backend" || len(g.Members) != 2 || g.Members[0] != "a" {
		t.Fatalf("group = %+v %v", g, err)
	}
	for _, bad := range []string{"x", "has space", "UPPER!", "here", "channel", "everyone"} {
		if _, err := domain.NewGroup("g", "ws", bad, "", nil, "a", t0); !errors.Is(err, domain.ErrInvalidHandle) {
			t.Fatalf("handle %q = %v", bad, err)
		}
	}
	if err := g.Change("qa-team", "QA", []domain.UserID{"c"}); err != nil || g.Handle != "qa-team" || g.Members[0] != "c" {
		t.Fatalf("change = %+v %v", g, err)
	}
	if got := domain.GroupMentions("ping @qa-team and @backend, not me@x.uz or @qa-team again"); len(got) != 2 || got[0] != "qa-team" || got[1] != "backend" {
		t.Fatalf("mentions = %v", got)
	}
}
