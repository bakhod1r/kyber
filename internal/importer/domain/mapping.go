package domain

import (
	"context"
	"errors"
)

// ErrAlreadyImported means the external key already has a Kyber issue in the project.
var ErrAlreadyImported = errors.New("issue already imported")

// Mapping links an issue of another tracker to the Kyber issue created from it.
type Mapping struct {
	Project     string // Kyber project key
	ExternalKey string // e.g. PROJ-12
	IssueKey    string // e.g. KYB-31
}

// MappingRepository remembers imports so re-running one never duplicates issues.
type MappingRepository interface {
	// Add stores a mapping; ErrAlreadyImported if (Project, ExternalKey) exists.
	Add(ctx context.Context, m Mapping) error
	// ForProject returns ExternalKey → IssueKey for the project.
	ForProject(ctx context.Context, project string) (map[string]string, error)
}
