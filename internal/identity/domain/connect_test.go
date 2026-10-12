package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func TestConnectToken(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	raw, tok := domain.NewConnectToken("u-1", now)
	if !strings.HasPrefix(raw, domain.ConnectPrefix) || len(raw) > 64 || strings.ContainsAny(raw, "+/=") {
		t.Fatalf("raw %q is not a valid Telegram start payload", raw)
	}
	if tok.Hash == raw || tok.Hash != domain.HashConnectToken(raw) || tok.User != "u-1" || !tok.ExpiresAt.Equal(now.Add(domain.ConnectTTL)) {
		t.Fatalf("token = %+v", tok)
	}
	other, _ := domain.NewConnectToken("u-1", now)
	if other == raw {
		t.Fatal("tokens must be random")
	}
	if !domain.IsConnectPayload(raw) || domain.IsConnectPayload("abc") {
		t.Fatal("payload detection")
	}
}
