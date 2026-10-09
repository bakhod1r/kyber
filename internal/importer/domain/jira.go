// Package domain is the Importer context: parsing a Jira CSV export and planning
// how each row becomes a Kyber issue. Pure functions; no I/O beyond the reader.
package domain

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Row is one Jira issue as exported (raw strings).
type Row struct {
	Line                                       int
	Key, Summary, Type, Status, StatusCategory string
	Priority, Assignee, Reporter, Created      string
	Resolved, Description, StoryPoints         string
}

var columns = map[string][]string{
	"summary":         {"summary"},
	"key":             {"issue key"},
	"type":            {"issue type"},
	"status":          {"status"},
	"status_category": {"status category"},
	"priority":        {"priority"},
	"assignee":        {"assignee"},
	"reporter":        {"reporter"},
	"created":         {"created"},
	"resolved":        {"resolved", "resolution date"},
	"description":     {"description"},
	"points":          {"story points", "custom field (story points)", "custom field (story point estimate)", "story point estimate"},
}

// ParseJiraCSV reads a Jira CSV export. Header names are matched case-insensitively;
// Jira repeats some columns (Labels, Sprint), so the first non-empty value wins.
func ParseJiraCSV(r io.Reader) ([]Row, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && string(b) == "\xef\xbb\xbf" {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	header, err := cr.Read()
	if err != nil {
		return nil, errors.New("the file is empty or not a CSV export")
	}
	index := map[string][]int{}
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		for field, aliases := range columns {
			for _, a := range aliases {
				if name == a {
					index[field] = append(index[field], i)
				}
			}
		}
	}
	for _, required := range []string{"summary", "key"} {
		if len(index[required]) == 0 {
			return nil, fmt.Errorf("missing required column %q (export from Jira with \"Export Excel CSV (all fields)\")",
				map[string]string{"summary": "Summary", "key": "Issue key"}[required])
		}
	}
	var rows []Row
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid CSV: %w", err)
		}
		line, _ := cr.FieldPos(0)
		get := func(field string) string {
			for _, i := range index[field] {
				if i < len(rec) {
					if v := strings.TrimSpace(rec[i]); v != "" {
						return v
					}
				}
			}
			return ""
		}
		rows = append(rows, Row{
			Line: line, Key: get("key"), Summary: get("summary"), Type: get("type"), Status: get("status"),
			StatusCategory: get("status_category"), Priority: get("priority"), Assignee: get("assignee"),
			Reporter: get("reporter"), Created: get("created"), Resolved: get("resolved"),
			Description: strings.ReplaceAll(get("description"), "\r\n", "\n"), StoryPoints: get("points"),
		})
	}
	return rows, nil
}

// MapType maps a Jira issue type; unknown types become "task" with a warning.
func MapType(s string) (string, bool) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "")) {
	case "bug":
		return "bug", true
	case "story", "user story":
		return "story", true
	case "task":
		return "task", true
	case "epic":
		return "epic", true
	case "subtask":
		return "subtask", true
	}
	return "task", false
}

// MapStatus prefers Jira's status category (To Do / In Progress / Done) and falls
// back to well-known status names; anything else becomes "todo" with a warning.
func MapStatus(status, category string) (string, bool) {
	switch strings.ToLower(category) {
	case "to do", "new":
		return "todo", true
	case "in progress", "indeterminate":
		return "in_progress", true
	case "done", "complete":
		return "done", true
	}
	switch strings.ToLower(status) {
	case "to do", "todo", "open", "backlog", "selected for development", "reopened", "new":
		return "todo", true
	case "in progress", "in review", "in development", "code review", "testing", "qa":
		return "in_progress", true
	case "done", "closed", "resolved", "complete", "completed", "released":
		return "done", true
	}
	return "todo", false
}

// MapPriority maps Jira priorities, including the legacy Blocker…Trivial scheme.
func MapPriority(s string) (string, bool) {
	switch strings.ToLower(s) {
	case "highest", "blocker", "critical":
		return "highest", true
	case "high", "major":
		return "high", true
	case "medium", "":
		return "medium", true
	case "low", "minor":
		return "low", true
	case "lowest", "trivial":
		return "lowest", true
	}
	return "medium", false
}

var timeLayouts = []string{
	"02/Jan/06 3:04 PM", "02/Jan/2006 3:04 PM", "02/Jan/06 15:04", "02/Jan/2006 15:04",
	"2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-07:00", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
}

// ParseJiraTime parses the date formats Jira exports (interpreted as UTC when no zone is given).
func ParseJiraTime(s string) (time.Time, bool) {
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// Member is a project member that assignees can be matched to.
type Member struct{ ID, Email, Name string }

// Item is a row that will be imported.
type Item struct {
	Row
	Type, Status, Priority string
	AssigneeID             string
	ReporterID             string // "" = not a project member; the importing user reports it
	Points                 *float64
	Created                time.Time
	Resolved               *time.Time
	Warnings               []string
}

type RowError struct {
	Line    int
	Key     string
	Message string
}

type ImportPlan struct {
	Items   []Item
	Errors  []RowError
	Skipped []string // Jira keys imported earlier
}

var jiraKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-[0-9]+$`)

// Plan validates and maps every row. alreadyImported holds Jira keys imported before.
func Plan(rows []Row, members []Member, alreadyImported map[string]bool) ImportPlan {
	var p ImportPlan
	seen := map[string]bool{}
	for _, r := range rows {
		fail := func(msg string) { p.Errors = append(p.Errors, RowError{Line: r.Line, Key: r.Key, Message: msg}) }
		switch {
		case !jiraKey.MatchString(r.Key):
			fail("missing or invalid Issue key")
			continue
		case seen[r.Key]:
			fail("duplicate Issue key in this file")
			continue
		case r.Summary == "":
			seen[r.Key] = true
			fail("missing Summary")
			continue
		}
		seen[r.Key] = true
		if alreadyImported[r.Key] {
			p.Skipped = append(p.Skipped, r.Key)
			continue
		}
		it := Item{Row: r}
		warn := func(format string, args ...any) { it.Warnings = append(it.Warnings, fmt.Sprintf(format, args...)) }
		var ok bool
		if it.Type, ok = MapType(r.Type); !ok {
			warn("unknown type %q imported as task", r.Type)
		}
		if it.Status, ok = MapStatus(r.Status, r.StatusCategory); !ok {
			warn("unknown status %q imported as To Do", r.Status)
		}
		if it.Priority, ok = MapPriority(r.Priority); !ok {
			warn("unknown priority %q imported as Medium", r.Priority)
		}
		if r.Assignee != "" {
			if it.AssigneeID = matchMember(r.Assignee, members); it.AssigneeID == "" {
				warn("assignee %q is not a project member; left unassigned", r.Assignee)
			}
		}
		it.ReporterID = matchMember(r.Reporter, members)
		if r.StoryPoints != "" {
			f, err := strconv.ParseFloat(r.StoryPoints, 64)
			if err != nil || f < 0 || f > 999 || math.Abs(f*10-math.Round(f*10)) > 1e-9 {
				warn("story points %q are not 0-999 in steps of 0.1; left unestimated", r.StoryPoints)
			} else {
				it.Points = &f
			}
		}
		if t, ok := ParseJiraTime(r.Created); ok {
			it.Created = t
		} else if r.Created != "" {
			warn("created date %q not understood; the import time is used", r.Created)
		}
		if t, ok := ParseJiraTime(r.Resolved); ok && it.Status == "done" {
			it.Resolved = &t
		}
		p.Items = append(p.Items, it)
	}
	return p
}

func matchMember(s string, members []Member) string {
	for _, m := range members {
		if strings.EqualFold(m.Email, s) || strings.EqualFold(m.Name, s) {
			return m.ID
		}
	}
	return ""
}
