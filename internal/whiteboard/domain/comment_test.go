package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

func TestElementComment(t *testing.T) {
	t0 := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	b, _ := domain.New("b-1", "ws", "c-1", "Architecture", "u-1", t0)
	c, err := b.Comment("k-1", "e-1", "u-2", " Is this cached? @u@x.uz ", t0)
	if err != nil || c.Body != "Is this cached? @u@x.uz" || c.Element != "e-1" || c.Conversation != "c-1" {
		t.Fatalf("comment = %+v %v", c, err)
	}
	ev := c.PullEvents()
	if len(ev) != 1 || ev[0].EventName() != "whiteboard.comment_added" || ev[0].Conversation != "c-1" || ev[0].Author != "u-2" || ev[0].Body != c.Body {
		t.Fatalf("events = %+v", ev)
	}
	for _, bad := range []struct{ el, body string }{{"e-1", " "}, {"e-1", strings.Repeat("x", 2001)}, {"../x", "hi"}} {
		if _, err := b.Comment("k", domain.ElementID(bad.el), "u", bad.body, t0); !errors.Is(err, domain.ErrInvalidComment) {
			t.Fatalf("%+v = %v", bad, err)
		}
	}
}
