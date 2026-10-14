package livehints_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/infrastructure/livehints"
	"github.com/bakhod1r/kyber/internal/platform/live"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type fakeBroadcaster struct {
	got []live.Change
	err error
}

func (f *fakeBroadcaster) Broadcast(_ context.Context, c live.Change) error {
	f.got = append(f.got, c)
	return f.err
}

type fakeLog struct{ warnings int }

func (l *fakeLog) WarnContext(context.Context, string, ...any) { l.warnings++ }

func TestPresenceChanged(t *testing.T) {
	b, log := &fakeBroadcaster{}, &fakeLog{}
	h := livehints.New(b, log)
	h.PresenceChanged(tenant.With(context.Background(), "w-1"), "KYB")
	want := live.Change{Workspace: "w-1", Project: "KYB", Event: "presence.changed"}
	if len(b.got) != 1 || b.got[0] != want || log.warnings != 0 {
		t.Fatalf("hint = %+v, warnings %d", b.got, log.warnings)
	}
	b.err = errors.New("down")
	h.PresenceChanged(context.Background(), "KYB")
	if log.warnings != 1 {
		t.Fatal("a failed hint is logged")
	}
}
