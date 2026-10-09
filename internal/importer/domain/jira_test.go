package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

// A trimmed but realistic "Export Excel CSV (all fields)" from Jira Cloud.
const jiraCSV = "\xef\xbb\xbf" + `Summary,Issue key,Issue id,Issue Type,Status,Status Category,Priority,Assignee,Reporter,Created,Resolved,Description,Labels,Labels,Custom field (Story Points)
"Login fails, on Safari",PROJ-1,10001,Bug,Closed,Done,Blocker,ali@x.uz,Bob Bek,09/Oct/25 1:42 PM,10/Oct/25 9:05 AM,"Steps:
1. open ""login""
2. fail",ui,,3
Board drag & drop,PROJ-2,10002,Story,In Review,In Progress,Major,Bob Bek,Bob Bek,2025-10-11 08:00,,,,,5.5
Write docs,PROJ-3,10003,Task,To Do,To Do,Trivial,Unknown Person,,11/Oct/25 10:00 AM,,,,,
,PROJ-4,10004,Task,To Do,To Do,Medium,,,11/Oct/25 10:00 AM,,,,,
Duplicate,PROJ-1,10005,Task,To Do,To Do,Medium,,,11/Oct/25 10:00 AM,,,,,
Weird,PROJ-6,10006,Improvement,Waiting,,Urgent!,,,not a date,,,,,abc
`

func members() []domain.Member {
	return []domain.Member{
		{ID: "u-ali", Email: "ali@x.uz", Name: "Ali Valiyev"},
		{ID: "u-bob", Email: "bob@x.uz", Name: "Bob Bek"},
	}
}

func TestParseJiraCSV(t *testing.T) {
	rows, err := domain.ParseJiraCSV(strings.NewReader(jiraCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("rows = %d", len(rows))
	}
	r := rows[0]
	if r.Key != "PROJ-1" || r.Summary != "Login fails, on Safari" || r.Type != "Bug" || r.StatusCategory != "Done" ||
		r.Description != "Steps:\n1. open \"login\"\n2. fail" || r.StoryPoints != "3" || r.Line != 2 {
		t.Fatalf("row 1 = %+v", r)
	}
	if rows[1].Line != 5 { // the multi-line description spans file lines 2-4
		t.Fatalf("row 2 line = %d", rows[1].Line)
	}
}

func TestParseRejectsFilesWithoutRequiredColumns(t *testing.T) {
	if _, err := domain.ParseJiraCSV(strings.NewReader("Title,Key\nx,y\n")); err == nil || !strings.Contains(err.Error(), "Summary") {
		t.Fatalf("err = %v", err)
	}
	if _, err := domain.ParseJiraCSV(strings.NewReader("")); err == nil {
		t.Fatal("empty file must be rejected")
	}
}

func TestPlan(t *testing.T) {
	rows, _ := domain.ParseJiraCSV(strings.NewReader(jiraCSV))
	plan := domain.Plan(rows, members(), map[string]bool{})

	if len(plan.Items) != 4 || len(plan.Errors) != 2 {
		t.Fatalf("items=%d errors=%+v", len(plan.Items), plan.Errors)
	}
	a := plan.Items[0]
	if a.Type != "bug" || a.Status != "done" || a.Priority != "highest" || a.AssigneeID != "u-ali" || a.Points == nil || *a.Points != 3 {
		t.Fatalf("PROJ-1 = %+v", a)
	}
	if !a.Created.Equal(time.Date(2025, 10, 9, 13, 42, 0, 0, time.UTC)) || a.Resolved == nil || !a.Resolved.Equal(time.Date(2025, 10, 10, 9, 5, 0, 0, time.UTC)) {
		t.Fatalf("dates = %v %v", a.Created, a.Resolved)
	}
	b := plan.Items[1]
	if b.Status != "in_progress" || b.Priority != "high" || b.AssigneeID != "u-bob" || *b.Points != 5.5 || b.Resolved != nil {
		t.Fatalf("PROJ-2 = %+v", b)
	}
	c := plan.Items[2]
	if c.Priority != "lowest" || c.AssigneeID != "" || !hasWarning(c, "assignee") {
		t.Fatalf("PROJ-3 = %+v", c)
	}
	w := plan.Items[3]
	if w.Type != "task" || w.Status != "todo" || w.Priority != "medium" || w.Points != nil ||
		!hasWarning(w, "type") || !hasWarning(w, "status") || !hasWarning(w, "priority") || !hasWarning(w, "points") || !hasWarning(w, "created") {
		t.Fatalf("PROJ-6 = %+v", w)
	}
	errs := map[string]string{}
	for _, e := range plan.Errors {
		errs[e.Key] = e.Message
	}
	if !strings.Contains(errs["PROJ-4"], "Summary") || !strings.Contains(errs["PROJ-1"], "duplicate") {
		t.Fatalf("errors = %+v", plan.Errors)
	}
}

func TestPlanSkipsAlreadyImported(t *testing.T) {
	rows, _ := domain.ParseJiraCSV(strings.NewReader(jiraCSV))
	plan := domain.Plan(rows, members(), map[string]bool{"PROJ-1": true, "PROJ-2": true})
	if len(plan.Items) != 2 || len(plan.Skipped) != 2 || plan.Skipped[0] != "PROJ-1" {
		t.Fatalf("items=%d skipped=%v", len(plan.Items), plan.Skipped)
	}
}

func TestMappingTables(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"Done", ""}: "done", {"Closed", ""}: "done", {"Resolved", ""}: "done", {"In Progress", ""}: "in_progress",
		{"In Review", ""}: "in_progress", {"Backlog", ""}: "todo", {"Open", ""}: "todo", {"Anything", "In Progress"}: "in_progress",
	} {
		if got, _ := domain.MapStatus(in[0], in[1]); got != want {
			t.Errorf("MapStatus(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
	for in, want := range map[string]string{"Sub-task": "subtask", "Subtask": "subtask", "EPIC": "epic", "story": "story"} {
		if got, _ := domain.MapType(in); got != want {
			t.Errorf("MapType(%q) = %q", in, got)
		}
	}
	for in, want := range map[string]string{"Critical": "highest", "Highest": "highest", "High": "high", "Minor": "low", "Low": "low", "Lowest": "lowest", "": "medium"} {
		if got, _ := domain.MapPriority(in); got != want {
			t.Errorf("MapPriority(%q) = %q", in, got)
		}
	}
	for _, s := range []string{"2025-10-09T13:42:00.000+0000", "09/Oct/2025 13:42", "2025-10-09 13:42:00"} {
		if got, ok := domain.ParseJiraTime(s); !ok || !got.Equal(time.Date(2025, 10, 9, 13, 42, 0, 0, time.UTC)) {
			t.Errorf("ParseJiraTime(%q) = %v %v", s, got, ok)
		}
	}
}

func hasWarning(it domain.Item, field string) bool {
	for _, w := range it.Warnings {
		if strings.Contains(w, field) {
			return true
		}
	}
	return false
}
