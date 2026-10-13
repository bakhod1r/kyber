package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/form/domain"
)

var questions = []domain.QuestionInput{{Label: " Steps to reproduce ", Required: true, Long: true}, {Label: "Browser"}}

func TestNewForm(t *testing.T) {
	f, err := domain.NewForm("f-1", "KYB", " Bug report ", "Tell us what broke.", "bug", questions)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Bug report" || f.IssueType != "bug" || len(f.Questions) != 2 ||
		f.Questions[0] != (domain.Question{ID: "q1", Label: "Steps to reproduce", Required: true, Long: true}) || f.Questions[1].ID != "q2" {
		t.Fatalf("form = %+v", f)
	}
	many := make([]domain.QuestionInput, 21)
	for i := range many {
		many[i] = domain.QuestionInput{Label: strings.Repeat("x", i+1)}
	}
	for _, tc := range []struct {
		name, intro, typ string
		qs               []domain.QuestionInput
	}{
		{"", "", "task", nil},
		{strings.Repeat("x", 81), "", "task", nil},
		{"Ok", strings.Repeat("x", 2001), "task", nil},
		{"Ok", "", "epic", nil},
		{"Ok", "", "subtask", nil},
		{"Ok", "", "task", many},
		{"Ok", "", "task", []domain.QuestionInput{{Label: " "}}},
		{"Ok", "", "task", []domain.QuestionInput{{Label: strings.Repeat("x", 121)}}},
		{"Ok", "", "task", []domain.QuestionInput{{Label: "Same"}, {Label: "same"}}},
	} {
		if _, err := domain.NewForm("f", "KYB", tc.name, tc.intro, tc.typ, tc.qs); !errors.Is(err, domain.ErrInvalidForm) {
			t.Errorf("%q/%s: %v", tc.name, tc.typ, err)
		}
	}
}

func TestRedesignKeepsFormOnError(t *testing.T) {
	f, _ := domain.NewForm("f-1", "KYB", "Bug report", "", "bug", questions)
	if err := f.Redesign("", "", "task", nil); !errors.Is(err, domain.ErrInvalidForm) || f.Name != "Bug report" || len(f.Questions) != 2 {
		t.Fatalf("redesign error changed the form: %v %+v", err, f)
	}
	if err := f.Redesign("Request", "Ask us", "story", nil); err != nil || f.IssueType != "story" || len(f.Questions) != 0 || f.Intro != "Ask us" {
		t.Fatalf("redesign = %+v %v", f, err)
	}
}

func TestSubmitTooLongForAnIssue(t *testing.T) {
	qs := make([]domain.QuestionInput, 5)
	answers := map[string]string{}
	for i := range qs {
		qs[i] = domain.QuestionInput{Label: strings.Repeat("q", i+1)}
		answers[fmt.Sprintf("q%d", i+1)] = strings.Repeat("a", 4500)
	}
	f, _ := domain.NewForm("f-1", "KYB", "Long", "", "task", qs)
	if _, err := f.Submit("Ok", answers); !errors.Is(err, domain.ErrInvalidSubmission) {
		t.Fatalf("22500 characters fit one issue: %v", err)
	}
}

func TestSubmit(t *testing.T) {
	f, _ := domain.NewForm("f-1", "KYB", "Bug report", "", "bug", questions)
	s, err := f.Submit(" Login fails ", map[string]string{"q1": "1. Open\n2. Click", "q2": " "})
	if err != nil {
		t.Fatal(err)
	}
	want := "Submitted with the form **Bug report**.\n\n**Steps to reproduce**\n\n1. Open\n2. Click\n\n**Browser**\n\n_No answer_"
	if s.Title != "Login fails" || s.Type != "bug" || s.Description != want {
		t.Fatalf("submission = %#v", s)
	}
	for _, tc := range []struct {
		summary string
		answers map[string]string
	}{
		{"", map[string]string{"q1": "x"}},
		{"Two\nlines", map[string]string{"q1": "x"}},
		{strings.Repeat("x", 256), map[string]string{"q1": "x"}},
		{"Ok", map[string]string{"q1": "  "}},
		{"Ok", map[string]string{"q1": "x", "q9": "?"}},
		{"Ok", map[string]string{"q1": strings.Repeat("x", 5001)}},
	} {
		if _, err := f.Submit(tc.summary, tc.answers); !errors.Is(err, domain.ErrInvalidSubmission) {
			t.Errorf("%q %v: %v", tc.summary, tc.answers, err)
		}
	}
}
