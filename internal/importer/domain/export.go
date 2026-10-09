package domain

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// ExportRow is one Kyber issue in Kyber vocabulary; Assignee/Reporter are emails.
type ExportRow struct {
	Key, Summary, Type, Status, Priority string
	Assignee, Reporter, Description      string
	Points                               *float64
}

var exportHeader = []string{"Issue key", "Summary", "Issue Type", "Status", "Status Category", "Priority",
	"Assignee", "Reporter", "Description", "Story Points"}

var (
	jiraType     = map[string]string{"bug": "Bug", "story": "Story", "task": "Task", "epic": "Epic", "subtask": "Sub-task"}
	jiraStatus   = map[string]string{"todo": "To Do", "in_progress": "In Progress", "done": "Done"}
	jiraPriority = map[string]string{"highest": "Highest", "high": "High", "medium": "Medium", "low": "Low", "lowest": "Lowest"}
)

// WriteJiraCSV writes issues as a Jira-compatible CSV (UTF-8 BOM so Excel opens it correctly)
// that ParseJiraCSV reads back unchanged.
func WriteJiraCSV(w io.Writer, rows []ExportRow) error {
	if _, err := io.WriteString(w, "\xef\xbb\xbf"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write(exportHeader); err != nil {
		return err
	}
	for _, r := range rows {
		points := ""
		if r.Points != nil {
			points = strconv.FormatFloat(*r.Points, 'f', -1, 64)
		}
		status := jiraStatus[r.Status]
		if err := cw.Write([]string{r.Key, safeCell(r.Summary), jiraType[r.Type], status, status, jiraPriority[r.Priority],
			r.Assignee, r.Reporter, safeCell(r.Description), points}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// safeCell stops spreadsheets from evaluating user text as a formula (CSV injection).
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
