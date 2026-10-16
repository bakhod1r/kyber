package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// A one-off email (an invitation) goes through the delivery queue, once per key.
func TestSendEmailQueuesOnce(t *testing.T) {
	ctx := context.Background()
	s, q, _ := deliverySetup(domain.ChannelEmail)
	for range 2 {
		if err := s.SendEmail(ctx, "ws-1", "dan@x.uz", "Join KYB", "link", "invite/i-1"); err != nil {
			t.Fatal(err)
		}
	}
	all := q.All()
	if len(all) != 1 || all[0].To != "dan@x.uz" || all[0].Channel != domain.ChannelEmail || all[0].Subject != "Join KYB" {
		t.Fatalf("queued = %+v", all)
	}
	none, _, _ := deliverySetup(domain.ChannelTelegram)
	if err := none.SendEmail(ctx, "ws-1", "dan@x.uz", "s", "b", "k"); !errors.Is(err, application.ErrNoEmail) {
		t.Fatalf("no SMTP err = %v", err)
	}
}
