package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// G9: one change applied to several issues; each issue succeeds or fails on its own (permissions, workflow).
func TestBulkChange(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 3)
	status, assignee, sprint := "in_progress", string(dev), "s-1"
	res, err := s.Bulk(ctx, dev, "KYB", []string{"KYB-1", "KYB-2", "KYB-9", "OPS-1"}, application.BulkChange{
		Status: &status, Assignee: &assignee, Sprint: &sprint, AddLabels: []string{"q4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 || res[0].Err != nil || res[1].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	if !errors.Is(res[2].Err, domain.ErrIssueNotFound) || !errors.Is(res[3].Err, domain.ErrIssueNotFound) {
		t.Fatalf("missing and other-project issues: %v, %v", res[2].Err, res[3].Err)
	}
	for _, k := range []string{"KYB-1", "KYB-2"} {
		is, _ := s.Get(ctx, dev, k)
		if is.Status() != domain.StatusInProgress || is.Assignee() != dev || is.Sprint() != "s-1" || !is.HasLabel("q4") {
			t.Fatalf("%s = status %s assignee %s sprint %s labels %v", k, is.Status(), is.Assignee(), is.Sprint(), is.Labels())
		}
	}
	if is, _ := s.Get(ctx, dev, "KYB-3"); is.Status() != domain.StatusTodo {
		t.Fatal("an issue outside the selection changed")
	}
}

func TestBulkGuards(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 2)
	status := "done"
	if _, err := s.Bulk(ctx, dev, "KYB", nil, application.BulkChange{Status: &status}); !errors.Is(err, application.ErrBulkInvalid) {
		t.Fatalf("no issues: %v", err)
	}
	if _, err := s.Bulk(ctx, dev, "KYB", []string{"KYB-1"}, application.BulkChange{}); !errors.Is(err, application.ErrBulkInvalid) {
		t.Fatalf("no change: %v", err)
	}
	if _, err := s.Bulk(ctx, dev, "KYB", slices.Repeat([]string{"KYB-1"}, 101), application.BulkChange{Status: &status}); !errors.Is(err, application.ErrBulkInvalid) {
		t.Fatalf("too many: %v", err)
	}
	// A viewer's changes are refused issue by issue.
	res, err := s.Bulk(ctx, viewer, "KYB", []string{"KYB-1", "KYB-2"}, application.BulkChange{Status: &status})
	if err != nil || res[0].Err == nil || res[1].Err == nil {
		t.Fatalf("viewer = %+v, %v", res, err)
	}
	// Duplicates are applied once.
	res, _ = s.Bulk(ctx, dev, "KYB", []string{"KYB-1", "KYB-1"}, application.BulkChange{Status: &status})
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("duplicates = %+v", res)
	}
}
