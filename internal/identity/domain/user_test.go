package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func TestParseEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"  Ali@Example.COM ", "ali@example.com", true},
		{"a@b.uz", "a@b.uz", true},
		{"noatsign", "", false},
		{"@x.com", "", false},
		{"a@", "", false},
		{"a b@x.com", "", false},
		{"Ali <ali@x.uz>", "", false}, // display-name form is not an address
		{"ali@@x.uz", "", false},
		{"ali@x..uz", "", false},
		{"J.Doe+tag@Gmail.com", "j.doe+tag@gmail.com", true}, // kept as typed, only lower-cased
		{strings.Repeat("a", 250) + "@x.com", "", false},
	}
	for _, tt := range tests {
		e, err := domain.ParseEmail(tt.in)
		if tt.ok != (err == nil) {
			t.Fatalf("ParseEmail(%q) err = %v", tt.in, err)
		}
		if err != nil && !errors.Is(err, domain.ErrInvalidEmail) {
			t.Fatalf("err = %v", err)
		}
		if tt.ok && e.String() != tt.want {
			t.Fatalf("got %q, want %q", e, tt.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := domain.ValidatePassword("short"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatalf("err = %v", err)
	}
	if err := domain.ValidatePassword(strings.Repeat("x", 200)); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatal("over-long password must be rejected (hash DoS)")
	}
	if err := domain.ValidatePassword("correct horse"); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestNewUser(t *testing.T) {
	e, _ := domain.ParseEmail("a@b.uz")
	if _, err := domain.NewUser("u-1", e, "  ", "hash"); !errors.Is(err, domain.ErrEmptyName) {
		t.Fatalf("err = %v", err)
	}
	u, err := domain.NewUser("u-1", e, " Ali ", "hash")
	if err != nil || u.Name() != "Ali" || u.Email() != e || u.PasswordHash() != "hash" || u.ID() != "u-1" {
		t.Fatalf("u = %+v, %v", u, err)
	}
}

func TestNameLimits(t *testing.T) {
	for _, bad := range []string{strings.Repeat("n", 101), "a\nb"} {
		if _, err := domain.NewUser("u", "a@b.uz", bad, "h"); !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("err = %v", err)
		}
	}
}
