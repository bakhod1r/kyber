package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]domain.Seconds{
		"30m": 1800, "1h": 3600, "1h 30m": 5400, "2d": 2 * 8 * 3600, "1w": 5 * 8 * 3600,
		"1w 2d 3h 4m": (5*8+2*8+3)*3600 + 240, " 2H ": 7200, "90": 90 * 60,
	} {
		got, err := domain.ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "h", "1x", "1h1h", "-1h", "1.5h", "0m", "9999w"} {
		if _, err := domain.ParseDuration(bad); !errors.Is(err, domain.ErrInvalidDuration) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	for s, want := range map[domain.Seconds]string{0: "0m", 1800: "30m", 5400: "1h 30m", 8 * 3600: "1d", (5*8+2*8+3)*3600 + 240: "1w 2d 3h 4m"} {
		if got := s.String(); got != want {
			t.Errorf("%d = %q, want %q", s, got, want)
		}
	}
}

func TestTimeTracking(t *testing.T) {
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo, Version: 1})
	if tt := is.TimeTracking(); tt != (domain.TimeTracking{}) {
		t.Fatalf("fresh = %+v", tt)
	}
	// Setting the original estimate also sets the remaining one while nothing is logged.
	is.EstimateTime(8 * 3600)
	if tt := is.TimeTracking(); tt.Original != 8*3600 || tt.Remaining != 8*3600 || tt.Spent != 0 {
		t.Fatalf("estimated = %+v", tt)
	}
	w, err := is.LogWork("w-1", "u-1", at, 3*3600, "  wrote the parser  ")
	if err != nil || w.Comment != "wrote the parser" || w.Seconds != 3*3600 || w.Author != "u-1" || !w.Started.Equal(at) {
		t.Fatalf("log = %+v %v", w, err)
	}
	if tt := is.TimeTracking(); tt.Spent != 3*3600 || tt.Remaining != 5*3600 || tt.Original != 8*3600 {
		t.Fatalf("after log = %+v", tt)
	}
	// Logging more than remains brings remaining to zero, never below.
	_, _ = is.LogWork("w-2", "u-2", at, 6*3600, "")
	if tt := is.TimeTracking(); tt.Spent != 9*3600 || tt.Remaining != 0 {
		t.Fatalf("over = %+v", tt)
	}
	// Changing the original estimate after work keeps remaining; it can be set by hand.
	is.EstimateTime(10 * 3600)
	is.EstimateRemaining(2 * 3600)
	if tt := is.TimeTracking(); tt.Original != 10*3600 || tt.Remaining != 2*3600 {
		t.Fatalf("re-estimated = %+v", tt)
	}
	for _, bad := range []domain.Seconds{0, -60} {
		if _, err := is.LogWork("w-3", "u-1", at, bad, ""); !errors.Is(err, domain.ErrInvalidDuration) {
			t.Fatalf("%d: %v", bad, err)
		}
	}
	if _, err := is.LogWork("w-3", "u-1", at, 60, string(make([]byte, 2001))); !errors.Is(err, domain.ErrInvalidWorklog) {
		t.Fatalf("long comment: %v", err)
	}
	// Removing a worklog gives its time back to remaining.
	is.RemoveWorklog(w, "u-1")
	if tt := is.TimeTracking(); tt.Spent != 6*3600 || tt.Remaining != 5*3600 {
		t.Fatalf("after remove = %+v", tt)
	}

	events := map[string]int{}
	for _, e := range is.PullEvents() {
		events[e.EventName()]++
	}
	if events["issue.work_logged"] != 2 || events["issue.worklog_removed"] != 1 {
		t.Fatalf("events = %v", events)
	}
	ch, _ := is.Changelog("u-1")
	fields := map[string]string{}
	for _, it := range ch.Items {
		fields[it.Field] = it.To
	}
	if fields["original_estimate"] != "1d 2h" || fields["remaining_estimate"] != "5h" || fields["time_spent"] != "6h" {
		t.Fatalf("changelog = %+v", ch.Items)
	}
	back := domain.Rehydrate(is.Snapshot())
	if back.TimeTracking() != is.TimeTracking() {
		t.Fatal("snapshot lost time tracking")
	}
}
