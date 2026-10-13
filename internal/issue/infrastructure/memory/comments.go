package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// CommentRepository stores comments in memory and appends events to the issue
// repository's outbox, mirroring the shared Postgres outbox table.
type CommentRepository struct {
	mu       sync.Mutex
	issues   *Repository
	comments []domain.Comment
}

func NewCommentRepository(issues *Repository) *CommentRepository {
	return &CommentRepository{issues: issues}
}

func (r *CommentRepository) Add(ctx context.Context, c *domain.Comment, events []domain.Event) error {
	r.mu.Lock()
	r.comments = append(r.comments, *domain.RehydrateComment(c.ID(), c.IssueID(), c.Author(), c.Body(), c.CreatedAt()))
	r.mu.Unlock()
	r.issues.appendOutbox(ctx, events)
	return nil
}

func (r *CommentRepository) ListByIssue(_ context.Context, issueID domain.IssueID) ([]*domain.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*domain.Comment{}
	for i := range r.comments {
		if r.comments[i].IssueID() == issueID {
			c := r.comments[i]
			out = append(out, &c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt().Before(out[j].CreatedAt()) })
	return out, nil
}
