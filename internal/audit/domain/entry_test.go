package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/audit/domain"
)

func TestNewEntry(t *testing.T) {
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.FixedZone("x", 3600))
	e, err := domain.NewEntry("u-1", "PUT /api/v1/projects/{key}/workflow", "/api/v1/projects/KYB/workflow", "KYB", 200, at)
	if err != nil {
		t.Fatal(err)
	}
	if e.Actor != "u-1" || e.Project != "KYB" || e.Status != 200 || e.At.Location() != time.UTC || !e.At.Equal(at) {
		t.Fatalf("entry = %+v", e)
	}
	cases := []struct {
		name          string
		actor, action string
		at            time.Time
	}{
		{"no actor", "", "POST /x", at},
		{"no action", "u-1", " ", at},
		{"no time", "u-1", "POST /x", time.Time{}},
	}
	for _, c := range cases {
		if _, err := domain.NewEntry(c.actor, c.action, "/x", "", 200, c.at); !errors.Is(err, domain.ErrInvalidEntry) {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}

// ProjectOf finds the project a request touched: a {key} path value or an issue key's prefix.
func TestProjectOf(t *testing.T) {
	cases := map[[2]string]string{
		{"KYB", ""}:      "KYB",
		{"", "OPS-12"}:   "OPS",
		{"", "bad"}:      "",
		{"", ""}:         "",
		{"KYB", "OPS-1"}: "KYB",
	}
	for in, want := range cases {
		if got := domain.ProjectOf(in[0], in[1]); got != want {
			t.Errorf("ProjectOf(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
