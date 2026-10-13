package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func TestNewPage(t *testing.T) {
	p, rev, err := domain.NewPage("p-1", "KYB", " Onboarding ", "# Hello", "u-1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Onboarding" || p.Body != "# Hello" || p.Author != "u-1" || p.UpdatedBy != "u-1" || !p.UpdatedAt.Equal(t0) {
		t.Fatalf("page = %+v", p)
	}
	if rev != (domain.Revision{Page: "p-1", Number: 1, Title: "Onboarding", Body: "# Hello", By: "u-1", At: t0}) {
		t.Fatalf("revision = %+v", rev)
	}
	for _, tc := range []struct{ title, body string }{
		{"  ", ""},
		{strings.Repeat("x", 201), ""},
		{"Ok", strings.Repeat("x", 100_001)},
		{"Two\nlines", ""},
	} {
		if _, _, err := domain.NewPage("p", "KYB", tc.title, tc.body, "u", t0); !errors.Is(err, domain.ErrInvalidPage) {
			t.Errorf("%q: %v", tc.title, err)
		}
	}
}

func TestRevise(t *testing.T) {
	p, _, _ := domain.NewPage("p-1", "KYB", "Onboarding", "v1", "u-1", t0)
	p.Version = 1
	if _, changed, err := p.Revise("", "x", "u-2", t0); !errors.Is(err, domain.ErrInvalidPage) || changed || p.Body != "v1" {
		t.Fatalf("invalid revise changes nothing: %v", err)
	}
	if _, changed, err := p.Revise("Onboarding", "v1", "u-2", t0.Add(time.Hour)); err != nil || changed {
		t.Fatalf("no change: %v %v", changed, err)
	}
	rev, changed, err := p.Revise("Onboarding guide", "v2", "u-2", t0.Add(time.Hour))
	if err != nil || !changed || rev.Number != 2 || rev.By != "u-2" || p.UpdatedBy != "u-2" || p.Author != "u-1" || p.Body != "v2" {
		t.Fatalf("revise = %+v %v %v / %+v", rev, changed, err, p)
	}
}
