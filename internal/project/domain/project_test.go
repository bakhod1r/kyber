package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

func TestNewProject(t *testing.T) {
	tests := []struct {
		name, key, title string
		wantErr          error
	}{
		{"valid", "KYB", "Kyber", nil},
		{"trims name", "AB1", "  Alpha  ", nil},
		{"bad key", "kyb", "Kyber", domain.ErrInvalidKey},
		{"empty name", "KYB", "  ", domain.ErrEmptyName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := domain.NewProject("p-1", tt.key, tt.title)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if p.ID() != "p-1" || p.Key() != tt.key || p.Name() == "" || p.Name() != trim(tt.title) {
				t.Fatalf("unexpected project %+v", p)
			}
		})
	}
}

func TestNextIssueNumber(t *testing.T) {
	p, _ := domain.NewProject("p-1", "KYB", "Kyber")
	for want := 1; want <= 3; want++ {
		if got := p.NextIssueNumber(); got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	}
}

func trim(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func TestRehydrate(t *testing.T) {
	p := domain.Rehydrate("p-1", "KYB", "Kyber", 41)
	if p.IssueSeq() != 41 || p.NextIssueNumber() != 42 {
		t.Fatalf("seq = %d", p.IssueSeq())
	}
}
