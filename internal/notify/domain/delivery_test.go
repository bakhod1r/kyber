package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func TestNewDelivery(t *testing.T) {
	d, err := domain.NewDelivery("d-1", "ws", domain.ChannelEmail, " a@x.uz ", "Subject", "Body", "n-1/email", t0)
	if err != nil {
		t.Fatal(err)
	}
	if d.To != "a@x.uz" || d.Status != domain.DeliveryPending || !d.NextAt.Equal(t0) || d.Attempts != 0 || d.Key != "n-1/email" {
		t.Fatalf("delivery = %+v", d)
	}
	bad := []struct {
		ch       domain.Channel
		to, body string
	}{
		{"fax", "a@x.uz", "b"},
		{domain.ChannelEmail, " ", "b"},
		{domain.ChannelTelegram, "123", " "},
	}
	for _, b := range bad {
		if _, err := domain.NewDelivery("d", "ws", b.ch, b.to, "s", b.body, "k", t0); !errors.Is(err, domain.ErrInvalidDelivery) {
			t.Errorf("%+v: err = %v", b, err)
		}
	}
	long := strings.Repeat("x", domain.MaxBody+10)
	d, _ = domain.NewDelivery("d", "ws", domain.ChannelTelegram, "1", "", long, "k", t0)
	if len([]rune(d.Body)) != domain.MaxBody {
		t.Fatalf("body not truncated: %d", len([]rune(d.Body)))
	}
}

// Retries back off exponentially (30 s, 1 m, 2 m … capped at 1 h); after MaxAttempts it is dead.
func TestDeliveryRetries(t *testing.T) {
	d, _ := domain.NewDelivery("d", "ws", domain.ChannelEmail, "a@x.uz", "s", "b", "k", t0)
	waits := []time.Duration{}
	now := t0
	for d.Status == domain.DeliveryPending {
		d.Failed("smtp: 451 try later", false, now)
		if d.Status == domain.DeliveryPending {
			waits = append(waits, d.NextAt.Sub(now))
			now = d.NextAt
		}
	}
	if d.Status != domain.DeliveryDead || d.Attempts != domain.MaxAttempts || d.LastError != "smtp: 451 try later" {
		t.Fatalf("after retries = %+v", d)
	}
	if len(waits) != domain.MaxAttempts-1 || waits[0] != 30*time.Second || waits[1] != time.Minute || waits[len(waits)-1] != time.Hour {
		t.Fatalf("backoff = %v", waits)
	}
}

func TestPermanentFailureAndSuccess(t *testing.T) {
	d, _ := domain.NewDelivery("d", "ws", domain.ChannelTelegram, "1", "", "b", "k", t0)
	d.Failed("telegram: bot was blocked by the user", true, t0)
	if d.Status != domain.DeliveryDead || d.Attempts != 1 {
		t.Fatalf("permanent = %+v", d)
	}
	ok, _ := domain.NewDelivery("d", "ws", domain.ChannelEmail, "a@x.uz", "s", "b", "k", t0)
	ok.Failed("timeout", false, t0)
	ok.Sent(t0.Add(time.Minute))
	if ok.Status != domain.DeliverySent || ok.Attempts != 2 || !ok.SentAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("sent = %+v", ok)
	}
}
