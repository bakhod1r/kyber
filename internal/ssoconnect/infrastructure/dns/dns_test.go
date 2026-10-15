package dns_test

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/dns"
)

func TestTXT(t *testing.T) {
	var deadline bool
	r := dns.Resolver{Lookup: func(ctx context.Context, name string) ([]string, error) {
		_, deadline = ctx.Deadline()
		return []string{"kyber-verify=" + name}, nil
	}}
	got, err := r.TXT(context.Background(), "acme.uz")
	if err != nil || len(got) != 1 || got[0] != "kyber-verify=acme.uz" || !deadline {
		t.Fatalf("txt = %v %v (deadline %v)", got, err, deadline)
	}
	// The system resolver answers (or fails) within the timeout; "invalid." never resolves.
	if _, err := (dns.Resolver{Timeout: 2 * time.Second}).TXT(context.Background(), "invalid."); err == nil {
		t.Fatal("invalid. resolved")
	}
}

func TestToken(t *testing.T) {
	a, b := dns.Token(), dns.Token()
	if len(a) != 32 || a == b {
		t.Fatalf("tokens %q %q", a, b)
	}
}
