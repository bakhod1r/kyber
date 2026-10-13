package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/call/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func TestStartJoinLeave(t *testing.T) {
	c := domain.Start("k-1", "w-1", "c-1", "u-a", t0)
	if !c.Live() || len(c.Participants) != 1 || c.Participants[0].User != "u-a" || c.Participants[0].Media != (domain.Media{Audio: true}) {
		t.Fatalf("started = %+v (the starter joins with the microphone on)", c)
	}
	if err := c.Join("u-b", t0.Add(time.Minute)); err != nil || !c.In("u-b") {
		t.Fatalf("join = %v", err)
	}
	if err := c.Join("u-b", t0); err != nil || len(c.Participants) != 2 {
		t.Fatalf("joining twice is a no-op: %v %d", err, len(c.Participants))
	}
	if err := c.Leave("u-c", t0); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("leave outsider = %v", err)
	}
	_ = c.Leave("u-a", t0.Add(2*time.Minute))
	if !c.Live() {
		t.Fatal("ends only when the last one leaves")
	}
	_ = c.Leave("u-b", t0.Add(3*time.Minute))
	if c.Live() || !c.EndedAt.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("ended = %+v", c)
	}
	if err := c.Join("u-a", t0); !errors.Is(err, domain.ErrCallEnded) {
		t.Fatalf("join ended = %v", err)
	}
}

func TestCallIsCapped(t *testing.T) {
	c := domain.Start("k-1", "w-1", "c-1", "u-0", t0)
	for i := 1; i < domain.MaxParticipants; i++ {
		if err := c.Join(domain.UserID(fmt.Sprint("u-", i)), t0); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Join("u-late", t0); !errors.Is(err, domain.ErrCallFull) {
		t.Fatalf("7th = %v", err)
	}
}

func TestMedia(t *testing.T) {
	c := domain.Start("k-1", "w-1", "c-1", "u-a", t0)
	if err := c.SetMedia("u-a", domain.Media{Audio: true, Video: true, Screen: true}); err != nil {
		t.Fatal(err)
	}
	if c.Participants[0].Media != (domain.Media{Audio: true, Video: true, Screen: true}) {
		t.Fatalf("media = %+v", c.Participants[0])
	}
	if err := c.SetMedia("u-x", domain.Media{}); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("outsider media = %v", err)
	}
	// Only one person shares a screen at a time (Zoom-style): a new share stops the previous one.
	_ = c.Join("u-b", t0)
	_ = c.SetMedia("u-b", domain.Media{Screen: true})
	if c.Participants[0].Media.Screen || !c.Participants[1].Media.Screen {
		t.Fatalf("screens = %+v", c.Participants)
	}
}

func TestSignal(t *testing.T) {
	c := domain.Start("k-1", "w-1", "c-1", "u-a", t0)
	_ = c.Join("u-b", t0)
	s, err := c.Signal("u-a", "u-b", "offer", `{"sdp":"v=0"}`)
	if err != nil || s.From != "u-a" || s.To != "u-b" || s.Kind != domain.SignalOffer || s.Call != "k-1" {
		t.Fatalf("signal = %+v %v", s, err)
	}
	cases := map[string]struct {
		from, to domain.UserID
		kind     string
		data     string
		want     error
	}{
		"outsider sends": {"u-x", "u-b", "offer", "{}", domain.ErrNotInCall},
		"to an outsider": {"u-a", "u-x", "offer", "{}", domain.ErrNotInCall},
		"to yourself":    {"u-a", "u-a", "offer", "{}", domain.ErrInvalidSignal},
		"unknown kind":   {"u-a", "u-b", "hack", "{}", domain.ErrInvalidSignal},
		"empty":          {"u-a", "u-b", "answer", "", domain.ErrInvalidSignal},
		"too large":      {"u-a", "u-b", "candidate", strings.Repeat("x", domain.MaxSignal+1), domain.ErrInvalidSignal},
	}
	for name, tc := range cases {
		if _, err := c.Signal(tc.from, tc.to, tc.kind, tc.data); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestPresentation(t *testing.T) {
	c := domain.Start("k-1", "w-1", "c-1", "u-a", t0)
	_ = c.Join("u-b", t0)
	if err := c.Present("u-x", domain.PresentDoc, "d-1"); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("outsider presents = %v", err)
	}
	for _, bad := range []struct{ kind, ref string }{{"slides", "x"}, {"doc", ""}, {"doc", strings.Repeat("x", 201)}} {
		if err := c.Present("u-a", domain.PresentKind(bad.kind), bad.ref); !errors.Is(err, domain.ErrInvalidPresentation) {
			t.Errorf("%v = %v", bad, err)
		}
	}
	if err := c.Present("u-a", domain.PresentDoc, "d-1"); err != nil || c.Presentation == nil || c.Presentation.By != "u-a" {
		t.Fatalf("present = %+v %v", c.Presentation, err)
	}
	// Someone else takes over (one presentation at a time).
	_ = c.Present("u-b", domain.PresentWhiteboard, "b-1")
	if c.Presentation.By != "u-b" || c.Presentation.Kind != domain.PresentWhiteboard {
		t.Fatalf("takeover = %+v", c.Presentation)
	}
	if err := c.StopPresenting("u-a"); !errors.Is(err, domain.ErrNotPresenter) {
		t.Fatalf("stop by someone else = %v", err)
	}
	if err := c.CanFollow("u-a"); !errors.Is(err, domain.ErrNotPresenter) {
		t.Fatalf("follow by non-presenter = %v", err)
	}
	if err := c.CanFollow("u-b"); err != nil {
		t.Fatal(err)
	}
	// The presenter leaving ends the presentation.
	_ = c.Leave("u-b", t0)
	if c.Presentation != nil {
		t.Fatalf("after presenter left = %+v", c.Presentation)
	}
	_ = c.Present("u-a", domain.PresentDoc, "d-1")
	if err := c.StopPresenting("u-a"); err != nil || c.Presentation != nil {
		t.Fatalf("stop = %v %+v", err, c.Presentation)
	}
}
