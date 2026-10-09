package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Event payloads are a published contract (outbox → webhooks): snake_case and stable.
func TestEventJSONContract(t *testing.T) {
	is := newIssue(t)
	_ = is.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
	ev := is.PullEvents()
	want := []string{
		`{"id":"i-1","key":"KYB-1","type":"task","title":"Login page","reporter":"u-reporter"}`,
		`{"id":"i-1","key":"KYB-1","from":"todo","to":"in_progress"}`,
	}
	for i, e := range ev {
		b, err := json.Marshal(e)
		if err != nil || string(b) != want[i] {
			t.Fatalf("%s = %s, want %s (%v)", e.EventName(), b, want[i], err)
		}
	}
}

func TestAllEventContracts(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	tests := []struct {
		e        domain.Event
		name     string
		wantJSON string
	}{
		{domain.IssueCreated{ID: "i-1", Key: key, Type: domain.TypeBug, Title: "t", Reporter: "u-1"}, "issue.created",
			`{"id":"i-1","key":"KYB-1","type":"bug","title":"t","reporter":"u-1"}`},
		{domain.IssueTransitioned{ID: "i-1", Key: key, From: "todo", To: "done"}, "issue.transitioned",
			`{"id":"i-1","key":"KYB-1","from":"todo","to":"done"}`},
		{domain.IssueEdited{ID: "i-1", Key: key, Fields: []string{"title"}}, "issue.edited",
			`{"id":"i-1","key":"KYB-1","fields":["title"]}`},
		{domain.IssueAssigned{ID: "i-1", Key: key, From: "", To: "u-1", By: "u-2"}, "issue.assigned",
			`{"id":"i-1","key":"KYB-1","from":"","to":"u-1","by":"u-2"}`},
		{domain.CommentAdded{ID: "c-1", IssueID: "i-1", IssueKey: key, Author: "u-1", Body: "hi @a@x.uz"}, "comment.added",
			`{"id":"c-1","issue_id":"i-1","issue_key":"KYB-1","author":"u-1","body":"hi @a@x.uz"}`},
	}
	for _, tt := range tests {
		if tt.e.EventName() != tt.name {
			t.Errorf("%T name = %q, want %q", tt.e, tt.e.EventName(), tt.name)
		}
		if b, _ := json.Marshal(tt.e); string(b) != tt.wantJSON {
			t.Errorf("%s json = %s, want %s", tt.name, b, tt.wantJSON)
		}
	}
}
