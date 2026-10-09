package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidKey      = errors.New("project key must be 2-10 uppercase letters/digits starting with a letter")
	ErrEmptyName       = errors.New("project name must not be empty")
	ErrProjectNotFound = errors.New("project not found")
	ErrKeyTaken        = errors.New("project key already taken")
)

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

type ProjectID string

// Project is the aggregate root of the Project context. It owns the issue number sequence.
type Project struct {
	id       ProjectID
	key      string
	name     string
	issueSeq int
}

func NewProject(id ProjectID, key, name string) (*Project, error) {
	if !keyRe.MatchString(key) {
		return nil, ErrInvalidKey
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	return &Project{id: id, key: key, name: name}, nil
}

// Rehydrate rebuilds a persisted project (repository use only).
func Rehydrate(id ProjectID, key, name string, issueSeq int) *Project {
	return &Project{id: id, key: key, name: name, issueSeq: issueSeq}
}

// IssueSeq is the last allocated issue number.
func (p *Project) IssueSeq() int { return p.issueSeq }

func (p *Project) ID() ProjectID { return p.id }
func (p *Project) Key() string   { return p.key }
func (p *Project) Name() string  { return p.name }

// NextIssueNumber advances and returns the project's issue sequence.
func (p *Project) NextIssueNumber() int {
	p.issueSeq++
	return p.issueSeq
}

// Repository is the persistence port for the Project aggregate.
type Repository interface {
	Create(ctx context.Context, p *Project) error // ErrKeyTaken
	// Update loads, mutates and stores atomically (row lock in Postgres).
	Update(ctx context.Context, key string, fn func(*Project) error) error
	ByKey(ctx context.Context, key string) (*Project, error) // ErrProjectNotFound
	List(ctx context.Context) ([]*Project, error)
}
