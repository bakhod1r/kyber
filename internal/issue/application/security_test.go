package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// A confidential issue is visible only to admins, its reporter and its assignee — everywhere.
func TestConfidentialIssues(t *testing.T) {
	ctx := context.Background()
	s, _, roles := setupWithAccess()
	roles["KYB"]["u-qa"] = "w" // another member
	epic, _ := s.Create(ctx, lead, application.CreateIssue{Project: "KYB", Title: "Epic", Type: "epic"})
	open, _ := s.Create(ctx, lead, application.CreateIssue{Project: "KYB", Title: "Open", Type: "task", Parent: epic.Key().String()})
	secret, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Salaries", Type: "task", Parent: epic.Key().String()})
	level := "confidential"
	if _, err := s.Edit(ctx, dev, secret.Key().String(), application.EditIssue{Version: secret.Version(), Security: &level}); err != nil {
		t.Fatal(err)
	}
	bad := "top-secret"
	if _, err := s.Edit(ctx, dev, open.Key().String(), application.EditIssue{Version: open.Version(), Security: &bad}); !errors.Is(err, application.ErrInvalidSecurityLevel) {
		t.Fatalf("unknown level err = %v", err)
	}
	_, _ = s.Link(ctx, lead, open.Key().String(), "relates", secret.Key().String())

	for who, sees := range map[string]bool{lead: true, dev: true, "u-qa": false, viewer: false} {
		_, err := s.Get(ctx, who, secret.Key().String())
		if sees != (err == nil) || (!sees && !errors.Is(err, application.ErrProjectNotFound)) {
			t.Errorf("%s get: %v", who, err)
		}
		list, _ := s.List(ctx, who, "KYB", application.ListQuery{})
		if found := contains(list, "Salaries"); found != sees {
			t.Errorf("%s list shows secret = %v", who, found)
		}
		kids, _ := s.Children(ctx, who, epic.Key().String())
		if found := contains(kids.Items, "Salaries"); found != sees || (sees && len(kids.Items) != 2) || (!sees && len(kids.Items) != 1) {
			t.Errorf("%s children = %d, secret %v", who, len(kids.Items), found)
		}
		links, _ := s.Links(ctx, who, open.Key().String())
		if (len(links) == 1) != sees {
			t.Errorf("%s links = %d", who, len(links))
		}
		if _, err := s.History(ctx, who, secret.Key().String()); sees != (err == nil) {
			t.Errorf("%s history err = %v", who, err)
		}
		if _, err := s.Comments(ctx, who, secret.Key().String()); sees != (err == nil) {
			t.Errorf("%s comments err = %v", who, err)
		}
	}
	// The assignee of a confidential issue sees it too.
	sec, _ := s.Get(ctx, dev, secret.Key().String())
	_, _ = s.Edit(ctx, dev, secret.Key().String(), application.EditIssue{Version: sec.Version(), AssigneeSet: true, Assignee: "u-qa"})
	if _, err := s.Get(ctx, "u-qa", secret.Key().String()); err != nil {
		t.Fatalf("assignee cannot see: %v", err)
	}
}

func contains(list []*domain.Issue, title string) bool {
	for _, is := range list {
		if is.Title() == title {
			return true
		}
	}
	return false
}
