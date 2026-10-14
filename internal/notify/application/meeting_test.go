package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

const scheduled = `{"id":"m-1","workspace":"00000000-0000-4000-8000-000000000001","project":"KYB","title":"Sprint review",
"start":"2026-10-17T15:00:00Z","end":"2026-10-17T16:00:00Z","attendees":["alice","bob","carol"],"organizer":"alice"}`

func TestMeetingScheduledTellsTheInvited(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnMeetingEvent(ctx, msg(20, "meeting.scheduled", scheduled)); err != nil {
		t.Fatal(err)
	}
	if got := inbox(t, s, "alice"); got != "" {
		t.Fatalf("the organizer is not notified: %q", got)
	}
	items, _, _ := s.List(ctx, "bob", false)
	if len(items) != 1 || items[0].Kind != domain.KindMeeting || items[0].IssueTitle != "Sprint review" || items[0].ActorName != "Alice" ||
		items[0].Excerpt != "Sat 17 Oct, 15:00–16:00 UTC" || items[0].Project != "KYB" {
		t.Fatalf("bob = %+v", items)
	}
}

func TestMeetingCancelledAndReminder(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	cancelled := strings.Replace(scheduled, `"organizer":"alice"`, `"by":"bob"`, 1)
	if err := s.OnMeetingEvent(ctx, msg(21, "meeting.cancelled", cancelled)); err != nil {
		t.Fatal(err)
	}
	if got := inbox(t, s, "alice"); got != "meeting::Bob" {
		t.Fatalf("alice = %q", got)
	}
	if got := inbox(t, s, "bob"); got != "" {
		t.Fatalf("the canceller is not notified: %q", got)
	}
	reminder := strings.Replace(scheduled, `,"organizer":"alice"`, "", 1)
	if err := s.OnMeetingEvent(ctx, msg(22, "meeting.reminder", reminder)); err != nil {
		t.Fatal(err)
	}
	items, _, _ := s.List(ctx, "alice", false)
	if len(items) != 2 || items[0].Excerpt != "Starts at 15:00 UTC" || items[0].ActorName != "" {
		t.Fatalf("everyone invited is reminded, the organizer too: %+v", items)
	}
	if err := s.OnMeetingEvent(ctx, msg(23, "meeting.reminder", `nope`)); err != nil {
		t.Fatalf("malformed events are skipped: %v", err)
	}
}

func TestMeetingEmailAndGroupPost(t *testing.T) {
	s, deliveries, _ := deliverySetup(domain.ChannelEmail)
	_ = s.OnMeetingEvent(context.Background(), msg(24, "meeting.scheduled", scheduled))
	all := deliveries.All()
	if len(all) != 2 { // bob and carol; never the organizer
		t.Fatalf("deliveries = %+v", all)
	}
	for _, d := range all {
		if !strings.HasSuffix(d.Body, "https://kyber.example/meetings") || d.Subject != "[KYB] Sprint review — Sat 17 Oct, 15:00–16:00 UTC" {
			t.Fatalf("delivery = %+v", d)
		}
	}

	g, _, gd, _ := groupSetup(admins{})
	linkGroup(t, g, "alice", "KYB", "-1001")
	if err := g.OnProjectEvent(context.Background(), msg(25, "meeting.reminder", scheduled)); err != nil {
		t.Fatal(err)
	}
	var posts []string
	for _, d := range gd.All() {
		if d.To == "-1001" {
			posts = append(posts, d.Body)
		}
	}
	if len(posts) != 1 || posts[0] != "📅 Sprint review — Starts at 15:00 UTC\nhttps://kyber.example/meetings" {
		t.Fatalf("group posts = %q", posts)
	}
	// Meetings of chosen people or the whole workspace are not posted to project groups.
	if err := g.OnProjectEvent(context.Background(), msg(26, "meeting.scheduled", strings.Replace(scheduled, `"project":"KYB",`, "", 1))); err != nil {
		t.Fatal(err)
	}
}

func TestMeetingTextInTheRecipientsTimeZone(t *testing.T) {
	ctx := context.Background()
	s, deliveries, prefs := deliverySetup(domain.ChannelEmail)
	_ = prefs.SetPrefs(ctx, "bob", domain.Prefs{Email: true, Telegram: true, TimeZone: "Asia/Tashkent"})
	if err := s.OnMeetingEvent(ctx, msg(27, "meeting.rescheduled", scheduled)); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range deliveries.All() {
		got[d.To] = d.Subject
	}
	if got["bob@x.uz"] != "[KYB] Sprint review — Moved to Sat 17 Oct, 20:00–21:00 Asia/Tashkent" ||
		got["carol@x.uz"] != "[KYB] Sprint review — Moved to Sat 17 Oct, 15:00–16:00 UTC" || len(got) != 2 {
		t.Fatalf("subjects = %q", got)
	}
}

func TestTimeZoneSetting(t *testing.T) {
	ctx := context.Background()
	s, _, _ := deliverySetup()
	if err := s.SetPrefs(ctx, "bob", domain.Prefs{TimeZone: "Mars/Base"}); !errors.Is(err, domain.ErrInvalidTimeZone) {
		t.Fatalf("err = %v", err)
	}
	if err := s.SetPrefs(ctx, "bob", domain.Prefs{Email: true, TimeZone: "Europe/Berlin"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Settings(ctx, "bob"); got.TimeZone != "Europe/Berlin" {
		t.Fatalf("settings = %+v", got)
	}
}
