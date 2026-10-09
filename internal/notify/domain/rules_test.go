package domain_test

import (
	"reflect"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func TestAssignmentRecipients(t *testing.T) {
	tests := []struct {
		to, by domain.UserID
		want   []domain.UserID
	}{
		{"bob", "alice", []domain.UserID{"bob"}},
		{"alice", "alice", nil}, // self-assignment: no notification
		{"", "alice", nil},      // unassigned
	}
	for _, tt := range tests {
		if got := domain.AssignmentRecipients(tt.to, tt.by); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("AssignmentRecipients(%q, %q) = %v, want %v", tt.to, tt.by, got, tt.want)
		}
	}
}

func TestCommentRecipients(t *testing.T) {
	tests := []struct {
		name                       string
		author, reporter, assignee domain.UserID
		mentioned                  []domain.UserID
		want                       map[domain.UserID]domain.Kind
	}{
		{"reporter and assignee", "carol", "alice", "bob", nil,
			map[domain.UserID]domain.Kind{"alice": domain.KindCommented, "bob": domain.KindCommented}},
		{"author never notified", "alice", "alice", "bob", []domain.UserID{"alice"},
			map[domain.UserID]domain.Kind{"bob": domain.KindCommented}},
		{"mention wins over commented", "carol", "alice", "bob", []domain.UserID{"bob", "dave"},
			map[domain.UserID]domain.Kind{"alice": domain.KindCommented, "bob": domain.KindMentioned, "dave": domain.KindMentioned}},
		{"reporter is assignee: one notification", "carol", "alice", "alice", nil,
			map[domain.UserID]domain.Kind{"alice": domain.KindCommented}},
		{"legacy issue without reporter", "carol", "", "", []domain.UserID{"dave", "dave"},
			map[domain.UserID]domain.Kind{"dave": domain.KindMentioned}},
	}
	for _, tt := range tests {
		got := domain.CommentRecipients(tt.author, tt.reporter, tt.assignee, tt.mentioned)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseMentions(t *testing.T) {
	tests := []struct {
		body string
		want []string
	}{
		{"hi @Bob@X.uz, please look", []string{"bob@x.uz"}},
		{"@a@x.uz and @b@y.com. Also @a@x.uz!", []string{"a@x.uz", "b@y.com"}},
		{"mail me at plain@x.uz (no @ prefix)", nil},
		{"(@c@z.io)", []string{"c@z.io"}},
		{"@not-an-email and @@x", nil},
	}
	for _, tt := range tests {
		if got := domain.ParseMentions(tt.body); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseMentions(%q) = %v, want %v", tt.body, got, tt.want)
		}
	}
}
