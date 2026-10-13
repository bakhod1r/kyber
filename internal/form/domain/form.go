// Package domain is the Forms context: intake forms of a project (Jira's Forms). Submitting one
// turns the answers into a new issue of the form's type.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidForm       = errors.New("invalid form")
	ErrInvalidSubmission = errors.New("invalid form submission")
	ErrFormNotFound      = errors.New("form not found")
	ErrFormConflict      = errors.New("form was changed concurrently")
)

const (
	maxName      = 80
	maxIntro     = 2000
	maxQuestions = 20
	maxLabel     = 120
	maxAnswer    = 5000
	maxSummary   = 255
	maxDesc      = 20_000 // an issue description's limit
)

type FormID string

// Question is one field of a form; IDs are q1, q2, … in display order.
type Question struct {
	ID       string
	Label    string
	Required bool
	Long     bool // a multi-line answer
}

// QuestionInput is a question as its designer writes it.
type QuestionInput struct {
	Label    string
	Required bool
	Long     bool
}

// Form creates issues of IssueType in its project. Every submission also asks for a summary,
// which becomes the issue's title.
type Form struct {
	ID        FormID
	Project   string
	Name      string
	Intro     string
	IssueType string
	Questions []Question
	Version   int
}

// NewForm designs a form.
func NewForm(id FormID, project, name, intro, issueType string, questions []QuestionInput) (*Form, error) {
	f := &Form{ID: id, Project: project}
	return f, f.Redesign(name, intro, issueType, questions)
}

// Redesign replaces the form's name, intro, issue type and questions; on error nothing changes.
func (f *Form) Redesign(name, intro, issueType string, questions []QuestionInput) error {
	name, intro = strings.TrimSpace(name), strings.TrimSpace(intro)
	switch {
	case name == "" || utf8.RuneCountInString(name) > maxName:
		return fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidForm, maxName)
	case utf8.RuneCountInString(intro) > maxIntro:
		return fmt.Errorf("%w: intro must be at most %d characters", ErrInvalidForm, maxIntro)
	case issueType != "story" && issueType != "task" && issueType != "bug":
		return fmt.Errorf("%w: a form creates stories, tasks or bugs", ErrInvalidForm)
	case len(questions) > maxQuestions:
		return fmt.Errorf("%w: at most %d questions", ErrInvalidForm, maxQuestions)
	}
	qs := make([]Question, 0, len(questions))
	seen := map[string]bool{}
	for i, q := range questions {
		label := strings.TrimSpace(q.Label)
		if label == "" || utf8.RuneCountInString(label) > maxLabel || seen[strings.ToLower(label)] {
			return fmt.Errorf("%w: each question needs a distinct label of 1-%d characters", ErrInvalidForm, maxLabel)
		}
		seen[strings.ToLower(label)] = true
		qs = append(qs, Question{ID: fmt.Sprintf("q%d", i+1), Label: label, Required: q.Required, Long: q.Long})
	}
	f.Name, f.Intro, f.IssueType, f.Questions = name, intro, issueType, qs
	return nil
}

// Submission is the issue a filled-in form becomes.
type Submission struct {
	Title       string
	Type        string
	Description string // Markdown: the form's name and every question with its answer
}

// Submit checks a filled-in form (summary plus answers by question id) and writes the issue.
func (f *Form) Submit(summary string, answers map[string]string) (Submission, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" || strings.ContainsAny(summary, "\r\n") || utf8.RuneCountInString(summary) > maxSummary {
		return Submission{}, fmt.Errorf("%w: the summary must be one line of 1-%d characters", ErrInvalidSubmission, maxSummary)
	}
	known := map[string]bool{}
	var b strings.Builder
	fmt.Fprintf(&b, "Submitted with the form **%s**.", f.Name)
	for _, q := range f.Questions {
		known[q.ID] = true
		a := strings.TrimSpace(answers[q.ID])
		switch {
		case a == "" && q.Required:
			return Submission{}, fmt.Errorf("%w: %q needs an answer", ErrInvalidSubmission, q.Label)
		case utf8.RuneCountInString(a) > maxAnswer:
			return Submission{}, fmt.Errorf("%w: answers must be at most %d characters", ErrInvalidSubmission, maxAnswer)
		case a == "":
			a = "_No answer_"
		}
		fmt.Fprintf(&b, "\n\n**%s**\n\n%s", q.Label, a)
	}
	for id := range answers {
		if !known[id] {
			return Submission{}, fmt.Errorf("%w: unknown question %q", ErrInvalidSubmission, id)
		}
	}
	if utf8.RuneCountInString(b.String()) > maxDesc {
		return Submission{}, fmt.Errorf("%w: the answers are too long for one issue (%d characters at most)", ErrInvalidSubmission, maxDesc)
	}
	return Submission{Title: summary, Type: f.IssueType, Description: b.String()}, nil
}

// Repository stores forms in the workspace of ctx.
type Repository interface {
	// Save inserts (Version 0) or updates with optimistic locking, then bumps Version.
	Save(ctx context.Context, f *Form) error
	ByID(ctx context.Context, id FormID) (*Form, error)
	// ForProject lists the project's forms by name.
	ForProject(ctx context.Context, project string) ([]*Form, error)
	Delete(ctx context.Context, id FormID) error
}
