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
		`{"id":"i-1","key":"KYB-1","type":"task","title":"Login page"}`,
		`{"id":"i-1","key":"KYB-1","from":"todo","to":"in_progress"}`,
	}
	for i, e := range ev {
		b, err := json.Marshal(e)
		if err != nil || string(b) != want[i] {
			t.Fatalf("%s = %s, want %s (%v)", e.EventName(), b, want[i], err)
		}
	}
}
