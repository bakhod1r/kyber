package domain_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

func TestReleaseNotes(t *testing.T) {
	got := domain.ReleaseNotes("1.0", "  The first one.  ", []domain.NoteIssue{
		{Key: "KYB-3", Title: "Crash on login", Type: "bug"},
		{Key: "KYB-1", Title: "Sign in", Type: "story"},
		{Key: "KYB-2", Title: "Old crash", Type: "bug"},
		{Key: "KYB-9", Title: "Spike", Type: "research"},
	})
	want := "# Release notes — 1.0\n\nThe first one.\n" +
		"\n## Stories\n\n- [KYB-1] Sign in\n" +
		"\n## Bug fixes\n\n- [KYB-2] Old crash\n- [KYB-3] Crash on login\n" +
		"\n## Other\n\n- [KYB-9] Spike\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if empty := domain.ReleaseNotes("2.0", "", nil); empty != "# Release notes — 2.0\n\nNo issues in this version.\n" {
		t.Fatalf("empty = %q", empty)
	}
}
