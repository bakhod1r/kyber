package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func TestPersonalToken(t *testing.T) {
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	later, past := at.Add(time.Hour), at
	tests := []struct {
		name    string
		expires *time.Time
		err     error
	}{
		{" MCP laptop ", nil, nil},
		{"CI", &later, nil},
		{"", nil, domain.ErrInvalidToken},
		{"a\nb", nil, domain.ErrInvalidToken},
		{strings.Repeat("x", 101), nil, domain.ErrInvalidToken},
		{"old", &past, domain.ErrInvalidToken},
	}
	for _, tt := range tests {
		tok, err := domain.NewPersonalToken("t", "u", tt.name, []byte("h"), at, tt.expires)
		if !errors.Is(err, tt.err) {
			t.Fatalf("%q: err = %v, want %v", tt.name, err, tt.err)
		}
		if err == nil && tok.Name != strings.TrimSpace(tt.name) {
			t.Fatalf("name = %q", tok.Name)
		}
	}
	tok, _ := domain.NewPersonalToken("t", "u", "CI", nil, at, &later)
	if tok.Expired(at) || !tok.Expired(later) {
		t.Fatal("expiry")
	}
	never, _ := domain.NewPersonalToken("t", "u", "CI", nil, at, nil)
	if never.Expired(at.Add(100 * 365 * 24 * time.Hour)) {
		t.Fatal("a token without expiry expired")
	}
	if !never.TouchDue(at) {
		t.Fatal("unused token not due")
	}
	used := at
	never.LastUsedAt = &used
	if never.TouchDue(at.Add(59*time.Second)) || !never.TouchDue(at.Add(time.Minute)) {
		t.Fatal("touch throttling")
	}
}
