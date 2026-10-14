package domain

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrInvalidLabel means a label is blank, has whitespace or a comma, is longer than 100
// characters, or the issue would carry more than 50 labels (Jira rules).
var ErrInvalidLabel = errors.New("labels must be 1-100 characters without spaces or commas, at most 50 per issue")

// Label is a free-form tag on an issue (Jira labels).
type Label string

const (
	maxLabel  = 100
	maxLabels = 50
)

// ParseLabel trims and validates one label.
func ParseLabel(s string) (Label, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > maxLabel || strings.ContainsRune(s, ',') || strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return "", ErrInvalidLabel
	}
	return Label(s), nil
}

// Labels are the issue's labels, sorted.
func (i *Issue) Labels() []Label { return slices.Clone(i.labels) }

// HasLabel reports whether the issue carries the label.
func (i *Issue) HasLabel(l Label) bool { return slices.Contains(i.labels, l) }

// Relabel replaces the issue's labels (duplicates are dropped); nothing changes on an invalid label.
func (i *Issue) Relabel(raw []string) error {
	out := make([]Label, 0, len(raw))
	for _, s := range raw {
		l, err := ParseLabel(s)
		if err != nil {
			return err
		}
		out = append(out, l)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > maxLabels {
		return ErrInvalidLabel
	}
	i.labels = out
	return nil
}

func joinLabels(ls []Label) string {
	s := make([]string, len(ls))
	for n, l := range ls {
		s[n] = string(l)
	}
	return strings.Join(s, ",")
}
