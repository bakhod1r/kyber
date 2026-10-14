package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/docs/domain"
)

func TestCommentOnParagraph(t *testing.T) {
	p, _, _ := domain.NewPage("p-1", "KYB", "Guide", "# Hi\n\nSecond", "u-1", t0)
	c, err := p.Comment("c-1", "ws-1", 1, "  Second ", "u-2", " Why? @ann@x.uz ", t0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Page != "p-1" || c.Project != "KYB" || c.Paragraph != 1 || c.Quote != "Second" || c.Body != "Why? @ann@x.uz" || c.Author != "u-2" {
		t.Fatalf("comment = %+v", c)
	}
	ev := c.PullEvents()
	if len(ev) != 1 || ev[0].EventName() != "doc.comment_added" || ev[0].Workspace != "ws-1" || ev[0].Project != "KYB" ||
		ev[0].Title != "Guide" || ev[0].Body != "Why? @ann@x.uz" || ev[0].Author != "u-2" || ev[0].Page != "p-1" {
		t.Fatalf("events = %+v", ev)
	}
	if len(c.PullEvents()) != 0 {
		t.Fatal("events pulled twice")
	}
	for _, tc := range []struct {
		para        int
		quote, body string
	}{
		{-1, "", "x"},
		{0, "", " "},
		{0, "", strings.Repeat("x", 2001)},
		{0, strings.Repeat("q", 201), "x"},
	} {
		if _, err := p.Comment("c", "ws", tc.para, tc.quote, "u", tc.body, t0); !errors.Is(err, domain.ErrInvalidComment) {
			t.Errorf("%d %q: %v", tc.para, tc.quote, err)
		}
	}
}
