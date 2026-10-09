package domain

import (
	"sort"
	"time"
)

// Day is one bucket of the created-vs-resolved report (UTC days).
type Day struct {
	Date                    time.Time
	Created, Resolved       int
	CumCreated, CumResolved int
}

// CreatedVsResolved buckets the last `days` UTC days ending today. A resolution is a
// transition into done (re-opening is not an un-resolution, as in Jira's report).
func CreatedVsResolved(entries []Entry, now time.Time, days int) []Day {
	today := now.UTC().Truncate(24 * time.Hour)
	first := today.AddDate(0, 0, -(days - 1))
	out := make([]Day, days)
	for i := range out {
		out[i].Date = first.AddDate(0, 0, i)
	}
	var baseC, baseR int
	for _, e := range entries {
		isCreated := e.Kind == KindCreated
		isResolved := e.Kind == KindStatus && e.To == done && e.From != done
		if !isCreated && !isResolved {
			continue
		}
		at := e.At.UTC()
		if at.After(now) {
			continue
		}
		idx := int(at.Sub(first).Hours() / 24)
		if at.Before(first) {
			if isCreated {
				baseC++
			} else {
				baseR++
			}
			continue
		}
		if idx >= days {
			continue
		}
		if isCreated {
			out[idx].Created++
		} else {
			out[idx].Resolved++
		}
	}
	c, r := baseC, baseR
	for i := range out {
		c += out[i].Created
		r += out[i].Resolved
		out[i].CumCreated, out[i].CumResolved = c, r
	}
	return out
}

// Cycle summarises cycle times (first "in progress" → done).
type Cycle struct {
	Count           int
	Average, Median time.Duration
}

// CycleTimes measures issues that reached done within (now-window, now].
func CycleTimes(entries []Entry, now time.Time, window time.Duration) Cycle {
	started := map[string]time.Time{}
	finished := map[string]time.Time{}
	for _, e := range sorted(entries) {
		if e.Kind != KindStatus || e.At.After(now) {
			continue
		}
		if e.To == "in_progress" {
			if _, ok := started[e.Issue]; !ok {
				started[e.Issue] = e.At
			}
		}
		if e.To == done {
			finished[e.Issue] = e.At
		} else if e.From == done {
			delete(finished, e.Issue) // re-opened: wait for the next done
		}
	}
	var ds []time.Duration
	for id, end := range finished {
		start, ok := started[id]
		if !ok || !end.After(now.Add(-window)) || end.Before(start) {
			continue
		}
		ds = append(ds, end.Sub(start))
	}
	if len(ds) == 0 {
		return Cycle{}
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	median := ds[len(ds)/2]
	if len(ds)%2 == 0 {
		median = (ds[len(ds)/2-1] + ds[len(ds)/2]) / 2
	}
	return Cycle{Count: len(ds), Average: sum / time.Duration(len(ds)), Median: median}
}

// Sample is a point of a burndown line (points in tenths).
type Sample struct {
	At              time.Time
	RemainingPoints int
	RemainingIssues int
}

type BurndownReport struct {
	StartPoints, StartIssues int
	Samples                  []Sample // a step line: one sample per change, plus start and "now"
	Ideal                    []Sample // straight line from StartPoints to 0 at the sprint end
	AddedPoints              int      // scope added after the start (joins + estimate increases)
	AddedIssues              int
	RemovedIssues            int
}

// Burndown replays the log: state at start, then every change touching the sprint up to `until`.
func Burndown(entries []Entry, sprint string, start, end, until time.Time) BurndownReport {
	st := replay{}
	all := sorted(entries)
	i := 0
	for ; i < len(all) && !all[i].At.After(start); i++ {
		st.apply(all[i])
	}
	var r BurndownReport
	r.StartPoints, r.StartIssues = st.remaining(sprint)
	r.Samples = []Sample{{At: start, RemainingPoints: r.StartPoints, RemainingIssues: r.StartIssues}}
	for ; i < len(all) && !all[i].At.After(until); i++ {
		e := all[i]
		before := *st.get(e.Issue)
		st.apply(e)
		after := *st.get(e.Issue)
		if before.sprint != sprint && after.sprint != sprint {
			continue
		}
		switch {
		case before.sprint != sprint && after.sprint == sprint:
			r.AddedIssues++
			r.AddedPoints += after.points
		case before.sprint == sprint && after.sprint != sprint:
			r.RemovedIssues++
		case after.points > before.points:
			r.AddedPoints += after.points - before.points
		}
		p, n := st.remaining(sprint)
		r.Samples = append(r.Samples, Sample{At: e.At, RemainingPoints: p, RemainingIssues: n})
	}
	if last := r.Samples[len(r.Samples)-1]; until.After(last.At) {
		r.Samples = append(r.Samples, Sample{At: until, RemainingPoints: last.RemainingPoints, RemainingIssues: last.RemainingIssues})
	}
	r.Ideal = []Sample{{At: start, RemainingPoints: r.StartPoints, RemainingIssues: r.StartIssues}, {At: end}}
	return r
}

// SprintWindow identifies a closed sprint for the velocity report.
type SprintWindow struct {
	ID, Name   string
	Start, End time.Time
}

type SprintVelocity struct {
	SprintWindow
	CommittedPoints int // in the sprint at its start
	CompletedPoints int // in the sprint and done at its end
	CompletedIssues int
}

func Velocity(entries []Entry, sprints []SprintWindow) []SprintVelocity {
	all := sorted(entries)
	out := make([]SprintVelocity, 0, len(sprints))
	for _, w := range sprints {
		st := replay{}
		i := 0
		for ; i < len(all) && !all[i].At.After(w.Start); i++ {
			st.apply(all[i])
		}
		v := SprintVelocity{SprintWindow: w}
		v.CommittedPoints, _ = st.remaining(w.ID)
		for ; i < len(all) && !all[i].At.After(w.End); i++ {
			st.apply(all[i])
		}
		for _, s := range st {
			if s.sprint == w.ID && s.status == done {
				v.CompletedPoints += s.points
				v.CompletedIssues++
			}
		}
		out = append(out, v)
	}
	return out
}
