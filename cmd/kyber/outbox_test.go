package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type fakeDead struct {
	list    []outbox.Message
	err     error
	retried []int64
}

func (f *fakeDead) DeadLetters(context.Context, int) ([]outbox.Message, error) { return f.list, f.err }

func (f *fakeDead) Retry(_ context.Context, id int64) error {
	if id == 404 {
		return outbox.ErrNotDead
	}
	f.retried = append(f.retried, id)
	return nil
}

func TestRunOutbox(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if err := runOutbox(ctx, []string{"dead-letters"}, &fakeDead{}, &out); err != nil || out.String() != "no dead letters\n" {
		t.Fatalf("empty = %q %v", out.String(), err)
	}
	out.Reset()
	f := &fakeDead{list: []outbox.Message{{ID: 7, Name: "issue.created", Workspace: "w-1", Actor: "u-1", At: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC), Attempts: 10, LastError: "boom"}}}
	if err := runOutbox(ctx, []string{"dead-letters"}, f, &out); err != nil || !strings.Contains(out.String(), "issue.created") || !strings.Contains(out.String(), "2026-10-13 09:00:00") {
		t.Fatalf("list = %q %v", out.String(), err)
	}
	out.Reset()
	if err := runOutbox(ctx, []string{"retry", "7"}, f, &out); err != nil || out.String() != "message 7 queued again\n" || f.retried[0] != 7 {
		t.Fatalf("retry = %q %v", out.String(), err)
	}
	if err := runOutbox(ctx, []string{"retry", "404"}, f, &out); !errors.Is(err, outbox.ErrNotDead) {
		t.Fatalf("retry live message: %v", err)
	}
	boom := errors.New("db down")
	if err := runOutbox(ctx, []string{"dead-letters"}, &fakeDead{err: boom}, &out); !errors.Is(err, boom) {
		t.Fatalf("store error: %v", err)
	}
	for _, args := range [][]string{nil, {"retry"}, {"retry", "x"}, {"purge"}} {
		if err := runOutbox(ctx, args, f, &out); err == nil || err.Error() != outboxUsage {
			t.Errorf("%v: %v", args, err)
		}
	}
}
