package domain

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

var ErrInvalidLink = errors.New("invalid development link")

// LinkKind is what a development link points at.
type LinkKind string

const (
	LinkCommit      LinkKind = "commit"
	LinkPullRequest LinkKind = "pull_request"
)

const (
	maxTitle = 500
	maxKeys  = 20 // issue keys taken from one commit or pull request
)

var issueKey = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-[1-9][0-9]*\b`)

// IssueKeys finds the distinct issue keys mentioned in texts (commit messages, PR titles,
// branch names), in order of appearance.
func IssueKeys(texts ...string) []string {
	var out []string
	for _, t := range texts {
		for _, k := range issueKey.FindAllString(t, -1) {
			if !slices.Contains(out, k) && len(out) < maxKeys {
				out = append(out, k)
			}
		}
	}
	return out
}

// Link is a commit or pull request that mentions an issue. Ref identifies it within its
// repository (commit SHA, PR number); saving the same issue, kind, repository and ref again
// updates it (a PR's title and state change).
type Link struct {
	Issue     string // issue key
	Kind      LinkKind
	Repo      string // "owner/name"
	Ref       string
	URL       string
	Title     string // first line of the commit message, or the PR title
	State     string // PR: open, closed or merged; commits: ""
	UpdatedAt time.Time
}

// NewLink describes a commit or PR; the title is cut to its first line.
func NewLink(issue string, kind LinkKind, repo, ref, url, title, state string, at time.Time) (Link, error) {
	if !issueKey.MatchString(issue) || (kind != LinkCommit && kind != LinkPullRequest) || repo == "" || ref == "" ||
		!(strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")) {
		return Link{}, ErrInvalidLink
	}
	title, _, _ = strings.Cut(strings.TrimSpace(title), "\n")
	if r := []rune(title); len(r) > maxTitle {
		title = string(r[:maxTitle-1]) + "…"
	}
	return Link{Issue: issue, Kind: kind, Repo: repo, Ref: ref, URL: url, Title: strings.TrimSpace(title), State: state, UpdatedAt: at}, nil
}

// LinkRepository stores development links in the workspace of ctx.
type LinkRepository interface {
	// Save inserts or updates the link of (issue, kind, repo, ref).
	Save(ctx context.Context, l Link) error
	// ForIssue lists an issue's links, most recently updated first.
	ForIssue(ctx context.Context, issue string) ([]Link, error)
}
