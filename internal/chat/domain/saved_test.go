package domain_test

import (
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestSavedReminder(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	m := domain.Message{ID: "m-1", Conversation: "c-1", Author: "u-2", Body: "Please review the deploy plan before standup tomorrow, it is long"}
	s := domain.Save("ws", "u-1", m, t0.Add(time.Hour), t0)
	if s.Remind(t0.Add(59 * time.Minute)) {
		t.Fatal("reminded before its time")
	}
	if !s.Remind(t0.Add(time.Hour)) || !s.Reminded {
		t.Fatal("not reminded at its time")
	}
	if s.Remind(t0.Add(2 * time.Hour)) {
		t.Fatal("reminded twice")
	}
	ev := s.PullEvents()
	if len(ev) != 1 || ev[0].User != "u-1" || ev[0].Message != "m-1" || ev[0].Conversation != "c-1" || ev[0].EventName() != "chat.reminder" ||
		len([]rune(ev[0].Excerpt)) > 60 {
		t.Fatalf("events = %+v", ev)
	}
	if none := domain.Save("ws", "u-1", m, time.Time{}, t0); none.Remind(t0.Add(time.Hour)) {
		t.Fatal("a save without a reminder time reminded")
	}
}
