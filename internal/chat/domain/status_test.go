package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestStatus(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	s, err := domain.NewStatus("ws", "u-1", "🏖", " On vacation ", t0.Add(time.Hour), t0.Add(30*time.Minute), t0)
	if err != nil || s.Text != "On vacation" {
		t.Fatalf("status = %+v %v", s, err)
	}
	if v := s.At(t0.Add(10 * time.Minute)); v.Emoji != "🏖" || !v.DND {
		t.Fatalf("at 9:10 = %+v", v)
	}
	if v := s.At(t0.Add(45 * time.Minute)); v.Emoji != "🏖" || v.DND {
		t.Fatalf("at 9:45 (DND over) = %+v", v)
	}
	if v := s.At(t0.Add(time.Hour)); v.Emoji != "" || v.Text != "" {
		t.Fatalf("expired status still shown: %+v", v)
	}
	if _, err := domain.NewStatus("ws", "u-1", "", strings.Repeat("x", 101), time.Time{}, time.Time{}, t0); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("long text = %v", err)
	}
	if _, err := domain.NewStatus("ws", "u-1", "too long emoji", "", time.Time{}, time.Time{}, t0); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("long emoji = %v", err)
	}
	if _, err := domain.NewStatus("ws", "u-1", "", "", t0.Add(-time.Minute), time.Time{}, t0); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expiry in the past = %v", err)
	}
	forever, _ := domain.NewStatus("ws", "u-1", "🎧", "Focus", time.Time{}, time.Time{}, t0)
	if v := forever.At(t0.Add(1000 * time.Hour)); v.Text != "Focus" || v.DND {
		t.Fatalf("no expiry = %+v", v)
	}
}
