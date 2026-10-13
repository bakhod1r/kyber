package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/goal/domain"
)

func TestNewGoal(t *testing.T) {
	g, err := domain.NewGoal("g-1", "KYB", "u-1", " Launch payments ", "2026-12-31", " project = KYB ")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Launch payments" || g.Status != domain.StatusOnTrack || g.Target != "2026-12-31" || g.JQL != "project = KYB" || g.Owner != "u-1" {
		t.Fatalf("goal = %+v", g)
	}
	for _, tc := range []struct{ name, target, jql string }{
		{"", "", ""},
		{strings.Repeat("x", 121), "", ""},
		{"Ok", "31.12.2026", ""},
		{"Ok", "", strings.Repeat("x", 2001)},
	} {
		if _, err := domain.NewGoal("g", "KYB", "u", tc.name, tc.target, tc.jql); !errors.Is(err, domain.ErrInvalidGoal) {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}

func TestRedefineAndReport(t *testing.T) {
	g, _ := domain.NewGoal("g-1", "KYB", "u-1", "Launch", "", "")
	if err := g.Redefine("", "", ""); !errors.Is(err, domain.ErrInvalidGoal) || g.Name != "Launch" {
		t.Fatalf("invalid redefine changes nothing: %v %q", err, g.Name)
	}
	if err := g.Redefine("Launch v2", "2027-01-15", "type = bug"); err != nil || g.Target != "2027-01-15" || g.JQL != "type = bug" {
		t.Fatalf("redefine: %v %+v", err, g)
	}
	for _, s := range []string{"on_track", "at_risk", "off_track", "done"} {
		if err := g.Report(domain.Status(s)); err != nil || string(g.Status) != s {
			t.Errorf("%s: %v", s, err)
		}
	}
	if err := g.Report("great"); !errors.Is(err, domain.ErrInvalidGoal) || g.Status != domain.StatusDone {
		t.Fatalf("unknown status: %v", err)
	}
}

func TestProgress(t *testing.T) {
	for _, tc := range []struct {
		p    domain.Progress
		want int
	}{
		{domain.Progress{}, 0},
		{domain.Progress{Done: 1, Total: 3}, 33},
		{domain.Progress{Done: 2, Total: 3}, 67},
		{domain.Progress{Done: 4, Total: 4}, 100},
	} {
		if got := tc.p.Percent(); got != tc.want {
			t.Errorf("%+v = %d, want %d", tc.p, got, tc.want)
		}
	}
}
