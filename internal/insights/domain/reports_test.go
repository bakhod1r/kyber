package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/domain"
)

var d0 = time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) // a Monday

func at(day, hour int) time.Time {
	return d0.Add(time.Duration(day)*24*time.Hour + time.Duration(hour)*time.Hour)
}
func pts(p int) *int { return &p }

func created(ev int64, t time.Time, issue string) domain.Entry {
	return domain.Entry{Event: ev, At: t, Project: "KYB", Issue: issue, Kind: domain.KindCreated, To: "todo"}
}
func status(ev int64, t time.Time, issue, from, to string) domain.Entry {
	return domain.Entry{Event: ev, At: t, Project: "KYB", Issue: issue, Kind: domain.KindStatus, From: from, To: to}
}
func sprint(ev int64, t time.Time, issue, from, to string) domain.Entry {
	return domain.Entry{Event: ev, At: t, Project: "KYB", Issue: issue, Kind: domain.KindSprint, From: from, To: to}
}
func estimate(ev int64, t time.Time, issue string, from, to *int) domain.Entry {
	return domain.Entry{Event: ev, At: t, Project: "KYB", Issue: issue, Kind: domain.KindEstimate, FromPoints: from, ToPoints: to}
}

func TestCreatedVsResolved(t *testing.T) {
	entries := []domain.Entry{
		created(1, at(0, 9), "a"), created(2, at(0, 10), "b"), created(3, at(2, 9), "c"),
		status(4, at(1, 9), "a", "todo", "in_progress"), status(5, at(2, 12), "a", "in_progress", "done"),
		status(6, at(2, 13), "b", "todo", "done"),
		status(7, at(3, 9), "b", "done", "in_progress"), // reopened: not a resolution
		created(8, at(-5, 9), "old"),                    // before the window: counts toward the starting totals only
	}
	days := domain.CreatedVsResolved(entries, at(3, 18), 4)
	want := []struct{ created, resolved, cumC, cumR int }{
		{2, 0, 3, 0}, {0, 0, 3, 0}, {1, 2, 4, 2}, {0, 0, 4, 2},
	}
	if len(days) != 4 {
		t.Fatalf("days = %d", len(days))
	}
	for i, w := range want {
		g := days[i]
		if !g.Date.Equal(at(i, 0)) || g.Created != w.created || g.Resolved != w.resolved || g.CumCreated != w.cumC || g.CumResolved != w.cumR {
			t.Errorf("day %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestCycleTime(t *testing.T) {
	entries := []domain.Entry{
		status(1, at(0, 0), "a", "todo", "in_progress"), status(2, at(2, 0), "a", "in_progress", "done"), // 48h
		status(3, at(1, 0), "b", "todo", "in_progress"), status(4, at(1, 12), "b", "in_progress", "todo"),
		status(5, at(3, 0), "b", "todo", "in_progress"), status(6, at(4, 0), "b", "in_progress", "done"), // first start → done: 72h
		status(7, at(4, 0), "c", "todo", "in_progress"),                                                          // not done: ignored
		status(8, at(-40, 0), "old", "todo", "in_progress"), status(9, at(-35, 0), "old", "in_progress", "done"), // outside window
	}
	ct := domain.CycleTimes(entries, at(5, 0), 30*24*time.Hour)
	if ct.Count != 2 || ct.Average != 60*time.Hour || ct.Median != 60*time.Hour {
		t.Fatalf("cycle = %+v", ct)
	}
	if empty := domain.CycleTimes(nil, at(5, 0), 24*time.Hour); empty.Count != 0 || empty.Average != 0 {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestBurndown(t *testing.T) {
	start, end := at(1, 9), at(5, 9)
	entries := []domain.Entry{
		// planned before the start: a (3 pts), b (5 pts), c (unestimated)
		created(1, at(0, 9), "a"), estimate(2, at(0, 9), "a", nil, pts(30)), sprint(3, at(0, 10), "a", "", "S"),
		created(4, at(0, 9), "b"), estimate(5, at(0, 9), "b", nil, pts(50)), sprint(6, at(0, 10), "b", "", "S"),
		created(7, at(0, 9), "c"), sprint(8, at(0, 10), "c", "", "S"),
		// during the sprint
		status(9, at(2, 9), "a", "todo", "done"), // -3
		created(10, at(2, 10), "d"), estimate(11, at(2, 10), "d", nil, pts(20)),
		sprint(12, at(2, 11), "d", "", "S"),           // scope +2
		estimate(13, at(3, 9), "b", pts(50), pts(80)), // re-estimate +3
		status(14, at(3, 12), "b", "todo", "done"),    // -8
		sprint(15, at(4, 9), "c", "S", ""),            // removed (0 pts, -1 issue)
		status(16, at(4, 10), "x", "todo", "done"),    // other issue: ignored
	}
	bd := domain.Burndown(entries, "S", start, end, at(4, 12))
	if bd.StartPoints != 80 || bd.StartIssues != 3 {
		t.Fatalf("start = %d pts / %d issues", bd.StartPoints, bd.StartIssues)
	}
	type s struct {
		t      time.Time
		points int
		issues int
	}
	want := []s{
		{start, 80, 3}, {at(2, 9), 50, 2}, {at(2, 11), 70, 3}, {at(3, 9), 100, 3},
		{at(3, 12), 20, 2}, {at(4, 9), 20, 1}, {at(4, 12), 20, 1},
	}
	if len(bd.Samples) != len(want) {
		t.Fatalf("samples = %+v", bd.Samples)
	}
	for i, w := range want {
		g := bd.Samples[i]
		if !g.At.Equal(w.t) || g.RemainingPoints != w.points || g.RemainingIssues != w.issues {
			t.Errorf("sample %d = %+v, want %+v", i, g, w)
		}
	}
	if bd.AddedPoints != 50 || bd.AddedIssues != 1 || bd.RemovedIssues != 1 {
		t.Fatalf("scope change = +%d pts / +%d / -%d issues", bd.AddedPoints, bd.AddedIssues, bd.RemovedIssues)
	}
	if len(bd.Ideal) != 2 || bd.Ideal[0].RemainingPoints != 80 || !bd.Ideal[1].At.Equal(end) || bd.Ideal[1].RemainingPoints != 0 {
		t.Fatalf("ideal = %+v", bd.Ideal)
	}
}

func TestVelocity(t *testing.T) {
	entries := []domain.Entry{
		estimate(1, at(0, 0), "a", nil, pts(30)), sprint(2, at(0, 1), "a", "", "S1"),
		estimate(3, at(0, 0), "b", nil, pts(50)), sprint(4, at(0, 1), "b", "", "S1"),
		status(5, at(3, 0), "a", "todo", "done"),
		estimate(6, at(4, 0), "c", nil, pts(20)), sprint(7, at(4, 0), "c", "", "S1"), // added mid-sprint, done
		status(8, at(5, 0), "c", "todo", "done"),
		sprint(9, at(7, 1), "b", "S1", ""), // returned at completion
	}
	v := domain.Velocity(entries, []domain.SprintWindow{{ID: "S1", Name: "Sprint 1", Start: at(1, 0), End: at(7, 0)}})
	if len(v) != 1 || v[0].CommittedPoints != 80 || v[0].CompletedPoints != 50 || v[0].CompletedIssues != 2 {
		t.Fatalf("velocity = %+v", v)
	}
}

func TestPointsHelpers(t *testing.T) {
	if math.Abs(domain.Tenths(25)-2.5) > 1e-9 {
		t.Fatal("Tenths")
	}
}
