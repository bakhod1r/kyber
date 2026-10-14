package ice_test

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/call/infrastructure/ice"
)

func TestServers(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cfg := ice.Config{URLs: []string{"stun:stun.example.com:3478", "turn:turn.example.com:3478?transport=udp", "turns:turn.example.com:5349"}, Secret: "s3cret", TTL: time.Hour}
	got := cfg.Servers("u-1", now)
	if len(got) != 2 || len(got[0].URLs) != 1 || got[0].Username != "" {
		t.Fatalf("servers = %+v (STUN without credentials, TURN together)", got)
	}
	turn := got[1]
	if len(turn.URLs) != 2 || turn.Username != "1700003600:u-1" {
		t.Fatalf("turn = %+v", turn)
	}
	mac := hmac.New(sha1.New, []byte("s3cret"))
	mac.Write([]byte(turn.Username))
	if turn.Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("credential must be coturn's use-auth-secret HMAC")
	}
	if got := (ice.Config{URLs: []string{"turn:t.example.com"}}).Servers("u-1", now); len(got) != 0 {
		t.Fatalf("TURN without a secret is useless: %+v", got)
	}
	if got := ice.Parse(" stun:a.example.com , ,turn:b.example.com "); len(got) != 2 || got[1] != "turn:b.example.com" {
		t.Fatalf("parse = %q", got)
	}
}
