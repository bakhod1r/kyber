// Package dns reads TXT records for SSO domain verification (ADR-0018 §4) and makes the tokens
// administrators publish there.
package dns

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"time"
)

// Resolver looks records up with the system resolver, bounded by Timeout.
type Resolver struct {
	Lookup  func(ctx context.Context, name string) ([]string, error) // nil: net.DefaultResolver
	Timeout time.Duration                                            // 0: 5 s
}

func (r Resolver) TXT(ctx context.Context, name string) ([]string, error) {
	lookup, timeout := r.Lookup, r.Timeout
	if lookup == nil {
		lookup = net.DefaultResolver.LookupTXT
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return lookup(ctx, name)
}

// Token is 16 random bytes in hex: what goes after "kyber-verify=".
func Token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails (crypto/rand, Go 1.24+)
	return hex.EncodeToString(b)
}
