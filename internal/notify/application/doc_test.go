package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

const docCommented = `{"id":"c-1","workspace":"00000000-0000-4000-8000-000000000001","project":"KYB","page_id":"p-1","title":"Guide","author_id":"alice","body":"@dave@x.uz @eve@x.uz @carol@x.uz look"}`

// S86: people @mentioned in a doc comment are told, if they are in the doc's project; never the author.
func TestDocCommentMentions(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnDocComment(ctx, msg(11, "doc.comment_added", docCommented)); err != nil {
		t.Fatal(err)
	}
	_ = s.OnDocComment(ctx, msg(11, "doc.comment_added", docCommented)) // redelivery
	for user, want := range map[string]string{"dave": "doc::Alice", "carol": "doc::Alice", "eve": "", "alice": "", "bob": ""} {
		if got := inbox(t, s, user); got != want {
			t.Errorf("%s = %q", user, got)
		}
	}
	items, _, _ := s.List(ctx, "dave", false)
	if items[0].Project != "KYB" || items[0].IssueTitle != "Guide" || !strings.Contains(items[0].Excerpt, "look") {
		t.Fatalf("dave = %+v", items[0])
	}
	if err := s.OnDocComment(ctx, msg(12, "doc.comment_added", `nope`)); err != nil {
		t.Fatalf("malformed events are skipped: %v", err)
	}
	if err := s.OnDocComment(ctx, msg(13, "doc.comment_added", strings.Replace(docCommented, `"KYB"`, `"BROKEN"`, 1))); err == nil {
		t.Fatal("audience failure is retried")
	}
}

func TestDocCommentEmailLinksToTheDocs(t *testing.T) {
	s, deliveries, _ := deliverySetup(domain.ChannelEmail)
	if err := s.OnDocComment(context.Background(), msg(14, "doc.comment_added", strings.Replace(docCommented, "@dave@x.uz", "@bob@x.uz", 1))); err != nil {
		t.Fatal(err)
	}
	all := deliveries.All()
	if len(all) == 0 || all[0].Subject != "[KYB] Alice mentioned you on «Guide»" || !strings.HasSuffix(all[0].Body, "https://kyber.example/projects/KYB/docs") {
		t.Fatalf("deliveries = %+v", all)
	}
}
