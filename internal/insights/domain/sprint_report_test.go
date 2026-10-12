package domain_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/domain"
)

func ids(l []domain.SprintIssue) string {
	s := ""
	for _, it := range l {
		s += fmt.Sprintf("%s:%d:%s:%v ", it.Issue, it.Points, it.Status, it.Added)
	}
	return s
}

func TestSprintReport(t *testing.T) {
	start, end := at(1, 9), at(5, 9)
	entries := []domain.Entry{
		// committed: a (3), b (5), c (0)
		created(1, at(0, 9), "a"), estimate(2, at(0, 9), "a", nil, pts(30)), sprint(3, at(0, 10), "a", "", "S"),
		created(4, at(0, 9), "b"), estimate(5, at(0, 9), "b", nil, pts(50)), sprint(6, at(0, 10), "b", "", "S"),
		created(7, at(0, 9), "c"), sprint(8, at(0, 10), "c", "", "S"),
		status(9, at(2, 9), "a", "todo", "done"),
		created(10, at(2, 10), "d"), estimate(11, at(2, 10), "d", nil, pts(20)), sprint(12, at(2, 11), "d", "", "S"), // added
		sprint(13, at(3, 9), "c", "S", ""),                                                                    // removed mid-sprint
		created(14, at(3, 10), "e"), sprint(15, at(3, 11), "e", "", "S"), sprint(16, at(3, 12), "e", "S", ""), // added, then removed
		status(17, at(4, 10), "x", "todo", "done"), // another issue
		// completion: unfinished b and d are carried over to the next sprint and the backlog
		carry(18, end, "b", "S", "S2"), carry(19, end, "d", "S", ""),
	}

	closed := domain.SprintReportOf(entries, "S", start, end)
	if closed.CommittedPoints != 80 || closed.CommittedIssues != 3 || closed.AddedIssues != 2 || closed.AddedPoints != 20 || closed.CompletedPoints != 30 {
		t.Fatalf("totals = %+v", closed)
	}
	if got := ids(closed.Completed); got != "a:30:done:false " {
		t.Errorf("completed = %s", got)
	}
	if got := ids(closed.NotCompleted); got != "b:50:todo:false d:20:todo:true " {
		t.Errorf("not completed (carried over) = %s", got)
	}
	if got := ids(closed.Removed); got != "c:0:todo:false e:0:todo:true " {
		t.Errorf("removed = %s", got)
	}

	// Before the completion, b and d are simply still in the sprint.
	active := domain.SprintReportOf(entries, "S", start, at(4, 12))
	if got := ids(active.NotCompleted); got != "b:50:todo:false d:20:todo:true " {
		t.Errorf("active not completed = %s", got)
	}
	// Carried back into the sprint later (a reopened plan) it is simply in the sprint again.
	again := append(entries, sprint(20, end.Add(1), "d", "", "S"))
	if got := ids(domain.SprintReportOf(again, "S", start, end.Add(2)).NotCompleted); got != "b:50:todo:false d:20:todo:true " {
		t.Errorf("rejoined = %s", got)
	}
}

func carry(ev int64, at time.Time, issue, from, to string) domain.Entry {
	return domain.Entry{Event: ev, At: at, Project: "KYB", Issue: issue, Kind: domain.KindCarried, From: from, To: to}
}
