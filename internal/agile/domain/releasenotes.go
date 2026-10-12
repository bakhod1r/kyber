package domain

import (
	"fmt"
	"slices"
	"strings"
)

// NoteIssue is an issue as release notes show it.
type NoteIssue struct {
	Key, Title, Type string
}

var noteSections = []struct{ typ, title string }{
	{"epic", "Epics"}, {"story", "Stories"}, {"bug", "Bug fixes"}, {"task", "Tasks"}, {"subtask", "Sub-tasks"},
}

// ReleaseNotes renders Jira-style release notes in Markdown: the version's issues grouped by type.
func ReleaseNotes(version, description string, issues []NoteIssue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Release notes — %s\n", version)
	if d := strings.TrimSpace(description); d != "" {
		fmt.Fprintf(&b, "\n%s\n", d)
	}
	if len(issues) == 0 {
		b.WriteString("\nNo issues in this version.\n")
		return b.String()
	}
	known := map[string]bool{}
	for _, sec := range noteSections {
		known[sec.typ] = true
		section(&b, sec.title, issues, func(t string) bool { return t == sec.typ })
	}
	section(&b, "Other", issues, func(t string) bool { return !known[t] })
	return b.String()
}

func section(b *strings.Builder, title string, issues []NoteIssue, match func(string) bool) {
	var in []NoteIssue
	for _, is := range issues {
		if match(is.Type) {
			in = append(in, is)
		}
	}
	if len(in) == 0 {
		return
	}
	slices.SortStableFunc(in, func(a, c NoteIssue) int { return strings.Compare(a.Key, c.Key) })
	fmt.Fprintf(b, "\n## %s\n\n", title)
	for _, is := range in {
		fmt.Fprintf(b, "- [%s] %s\n", is.Key, is.Title)
	}
}
