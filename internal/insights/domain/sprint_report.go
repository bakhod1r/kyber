package domain

import (
	"sort"
	"time"
)

// SprintIssue is an issue in a sprint report (points in tenths).
type SprintIssue struct {
	Issue  string // issue id
	Points int
	Status string // status category
	Added  bool   // joined after the sprint started
}

// SprintReport is Jira's sprint report: what was committed, added, completed, left undone or removed.
type SprintReport struct {
	CommittedPoints, CommittedIssues int // in the sprint at its start
	AddedPoints, AddedIssues         int // joined after the start (points as of the report)
	CompletedPoints                  int
	Completed, NotCompleted, Removed []SprintIssue
}

// SprintReportOf replays the log for the sprint from its start up to `until` (now, or the moment it
// was completed). Issues carried over by the completion count as not completed, not as removed.
func SprintReportOf(entries []Entry, sprint string, start, until time.Time) SprintReport {
	st := replay{}
	all := sorted(entries)
	i := 0
	for ; i < len(all) && !all[i].At.After(start); i++ {
		st.apply(all[i])
	}
	var r SprintReport
	committed := map[string]bool{}
	for id, s := range st {
		if s.sprint == sprint {
			committed[id] = true
		}
	}
	r.CommittedPoints, r.CommittedIssues = st.remainingAll(sprint)
	touched := map[string]bool{}
	for id := range committed {
		touched[id] = true
	}
	carried := map[string]bool{}
	for ; i < len(all) && !all[i].At.After(until); i++ {
		e := all[i]
		st.apply(e)
		switch {
		case st.get(e.Issue).sprint == sprint:
			touched[e.Issue], carried[e.Issue] = true, false
		case e.Kind == KindCarried && e.From == sprint:
			carried[e.Issue] = true
		}
	}
	for id := range touched {
		s := st.get(id)
		it := SprintIssue{Issue: id, Points: s.points, Status: s.status, Added: !committed[id]}
		if it.Added {
			r.AddedIssues++
			r.AddedPoints += s.points
		}
		switch {
		case s.sprint != sprint && !carried[id]:
			r.Removed = append(r.Removed, it)
		case s.status == done:
			r.Completed = append(r.Completed, it)
			r.CompletedPoints += s.points
		default:
			r.NotCompleted = append(r.NotCompleted, it)
		}
	}
	for _, l := range [][]SprintIssue{r.Completed, r.NotCompleted, r.Removed} {
		sort.Slice(l, func(a, b int) bool { return l[a].Issue < l[b].Issue })
	}
	return r
}

// remainingAll sums every member of sprint, done or not.
func (r replay) remainingAll(sprint string) (points, issues int) {
	for _, s := range r {
		if s.sprint == sprint {
			points += s.points
			issues++
		}
	}
	return
}
