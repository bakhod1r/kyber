package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestBookmark(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	b, err := domain.NewBookmark("b-1", "ws", "c-1", domain.BookmarkLink, " https://ci.example/builds ", " CI ", "u-1", t0)
	if err != nil || b.Ref != "https://ci.example/builds" || b.Title != "CI" {
		t.Fatalf("link = %+v %v", b, err)
	}
	if _, err := domain.NewBookmark("b-2", "ws", "c-1", domain.BookmarkDoc, "75000000-0000-4000-8000-000000000001", "Design", "u-1", t0); err != nil {
		t.Fatalf("doc = %v", err)
	}
	for _, bad := range []struct {
		kind       domain.BookmarkKind
		ref, title string
	}{
		{domain.BookmarkLink, "javascript:alert(1)", "x"},
		{domain.BookmarkLink, "ftp://x", "x"},
		{domain.BookmarkLink, "https://x", ""},
		{domain.BookmarkLink, "https://x", strings.Repeat("t", 101)},
		{"video", "x", "x"},
		{domain.BookmarkDoc, "", "x"},
	} {
		if _, err := domain.NewBookmark("b", "ws", "c", bad.kind, bad.ref, bad.title, "u", t0); !errors.Is(err, domain.ErrInvalidBookmark) {
			t.Fatalf("%+v = %v", bad, err)
		}
	}
}
