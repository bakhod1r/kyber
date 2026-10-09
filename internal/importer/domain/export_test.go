package domain_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

func TestExportRoundTrips(t *testing.T) {
	three := 2.5
	in := []domain.ExportRow{
		{Key: "KYB-1", Summary: "Login, \"fails\"", Type: "subtask", Status: "in_progress", Priority: "highest",
			Assignee: "ali@kyber.dev", Reporter: "lead@kyber.dev", Description: "line 1\nline 2", Points: &three},
		{Key: "KYB-2", Summary: "=HYPERLINK(\"x\")", Type: "epic", Status: "done", Priority: "lowest"},
	}
	var buf bytes.Buffer
	if err := domain.WriteJiraCSV(&buf, in); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "\xef\xbb\xbfIssue key,Summary,Issue Type,Status,Status Category,Priority,Assignee,Reporter,Description,Story Points\r\n") {
		t.Fatalf("header = %q", strings.SplitN(buf.String(), "\n", 2)[0])
	}
	if !strings.Contains(buf.String(), `'=HYPERLINK`) {
		t.Fatal("the exported file must defuse formulas")
	}
	rows, err := domain.ParseJiraCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}
	members := []domain.Member{{ID: "u-ali", Email: "ali@kyber.dev"}, {ID: "u-lead", Email: "lead@kyber.dev"}}
	plan := domain.Plan(rows, members, nil)
	if len(plan.Items) != 2 || len(plan.Errors) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	a, b := plan.Items[0], plan.Items[1]
	if a.Summary != `Login, "fails"` || a.Type != "subtask" || a.Status != "in_progress" || a.Priority != "highest" ||
		a.AssigneeID != "u-ali" || a.ReporterID != "u-lead" || a.Description != "line 1\nline 2" || a.Points == nil || *a.Points != 2.5 || len(a.Warnings) != 0 {
		t.Fatalf("round trip a = %+v", a)
	}
	// Export neutralises formulas with a leading apostrophe; import removes it again (QA-3).
	if b.Summary != "=HYPERLINK(\"x\")" || b.Type != "epic" || b.Status != "done" || b.Priority != "lowest" || b.Points != nil || b.ReporterID != "" {
		t.Fatalf("round trip b = %+v", b)
	}
}
