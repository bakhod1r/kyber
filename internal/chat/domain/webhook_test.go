package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestWebhook(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	h, err := domain.NewWebhook("h-1", "ws", "c-1", " CI ", "secret-1", "u-1", t0)
	if err != nil || h.Name != "CI" || h.SecretHash == "secret-1" || h.SecretHash != domain.HashSecret("secret-1") {
		t.Fatalf("hook = %+v %v", h, err)
	}
	h.Rotate("secret-2")
	if h.SecretHash != domain.HashSecret("secret-2") {
		t.Fatal("rotate kept the old secret")
	}
	if _, err := domain.NewWebhook("h", "ws", "c", "", "s", "u", t0); !errors.Is(err, domain.ErrInvalidWebhook) {
		t.Fatalf("no name = %v", err)
	}
}
