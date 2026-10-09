package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestParseIssueKey(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"KYB-12", "KYB-12", false},
		{"AB2-1", "AB2-1", false},
		{"KYB", "", true},
		{"KYB-", "", true},
		{"KYB-x", "", true},
		{"kyb-1", "", true},
		{"KYB-0", "", true},
		{"KYB-1-2", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			k, err := domain.ParseIssueKey(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && k.String() != tt.want {
				t.Fatalf("got %q, want %q", k, tt.want)
			}
		})
	}
}

func TestParseIssueType(t *testing.T) {
	for _, s := range []string{"epic", "story", "task", "bug", "subtask"} {
		if typ, err := domain.ParseIssueType(s); err != nil || string(typ) != s {
			t.Fatalf("ParseIssueType(%q) = %q, %v", s, typ, err)
		}
	}
	if _, err := domain.ParseIssueType("feature"); !errors.Is(err, domain.ErrInvalidIssueType) {
		t.Fatalf("err = %v, want ErrInvalidIssueType", err)
	}
}

func TestDefaultWorkflow(t *testing.T) {
	wf := domain.DefaultWorkflow()
	if wf.Initial() != domain.StatusTodo {
		t.Fatalf("initial = %q", wf.Initial())
	}
	allowed := []struct{ from, to domain.StatusID }{
		{domain.StatusTodo, domain.StatusInProgress},
		{domain.StatusInProgress, domain.StatusTodo},
		{domain.StatusInProgress, domain.StatusDone},
		{domain.StatusDone, domain.StatusInProgress},
	}
	for _, a := range allowed {
		if !wf.CanTransition(a.from, a.to) {
			t.Errorf("%s -> %s should be allowed", a.from, a.to)
		}
	}
	if wf.CanTransition(domain.StatusTodo, domain.StatusDone) {
		t.Error("todo -> done must be rejected")
	}
}
