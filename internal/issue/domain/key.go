package domain

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidProjectKey  = errors.New("project key must be 2-10 uppercase letters/digits starting with a letter")
	ErrInvalidIssueNumber = errors.New("issue number must be positive")
)

var projectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// IssueKey is the human-readable identifier of an issue, e.g. "KYB-12".
type IssueKey struct {
	project string
	number  int
}

func NewIssueKey(project string, number int) (IssueKey, error) {
	if !projectKeyRe.MatchString(project) {
		return IssueKey{}, ErrInvalidProjectKey
	}
	if number <= 0 {
		return IssueKey{}, ErrInvalidIssueNumber
	}
	return IssueKey{project: project, number: number}, nil
}

func (k IssueKey) Project() string { return k.project }
func (k IssueKey) Number() int     { return k.number }
func (k IssueKey) String() string  { return fmt.Sprintf("%s-%d", k.project, k.number) }
