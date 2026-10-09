package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

// ParseIssueKey parses the "PROJECT-NUMBER" form, e.g. "KYB-12".
func ParseIssueKey(s string) (IssueKey, error) {
	i := strings.LastIndexByte(s, '-')
	if i < 0 {
		return IssueKey{}, ErrInvalidProjectKey
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return IssueKey{}, ErrInvalidIssueNumber
	}
	return NewIssueKey(s[:i], n)
}

// MarshalText renders the key as "KYB-12" (used in event payloads and JSON).
func (k IssueKey) MarshalText() ([]byte, error) { return []byte(k.String()), nil }
