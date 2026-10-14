package application

import (
	"context"
	"strconv"

	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

// Commit is one commit of a GitHub push.
type Commit struct{ SHA, URL, Message string }

// Push is a GitHub push event.
type Push struct {
	Repo    string // "owner/name"
	Commits []Commit
}

// PullRequest is a GitHub pull_request event.
type PullRequest struct {
	Repo   string
	Number int
	URL    string
	Title  string
	Branch string // head ref
	State  string // open or closed
	Merged bool
}

// RecordPush links every commit to the issue keys in its message and returns how many links it saved.
func (s *Service) RecordPush(ctx context.Context, p Push) (int, error) {
	n := 0
	for _, c := range p.Commits {
		saved, err := s.link(ctx, domain.IssueKeys(c.Message), domain.LinkCommit, p.Repo, c.SHA, c.URL, c.Message, "")
		n += saved
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// RecordPullRequest links the pull request to the issue keys in its title and branch name.
func (s *Service) RecordPullRequest(ctx context.Context, pr PullRequest) (int, error) {
	state := pr.State
	if pr.Merged {
		state = "merged"
	}
	return s.link(ctx, domain.IssueKeys(pr.Title, pr.Branch), domain.LinkPullRequest, pr.Repo, strconv.Itoa(pr.Number), pr.URL, pr.Title, state)
}

// link saves one link per key. Keys of issues that do not exist are kept too: the links are only
// shown through the issue's own authorization (Development), never listed on their own.
func (s *Service) link(ctx context.Context, keys []string, kind domain.LinkKind, repo, ref, url, title, state string) (int, error) {
	n := 0
	for _, k := range keys {
		l, err := domain.NewLink(k, kind, repo, ref, url, title, state, s.d.Now().UTC())
		if err != nil {
			return n, err
		}
		if err := s.d.Links.Save(ctx, l); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
