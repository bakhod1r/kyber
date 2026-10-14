package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// ReactionRepository keeps comment reactions in memory (comment ids are globally unique).
type ReactionRepository struct {
	mu   sync.Mutex
	list []domain.Reaction // oldest first
}

func NewReactionRepository() *ReactionRepository { return &ReactionRepository{} }

func (r *ReactionRepository) React(_ context.Context, re domain.Reaction, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !slices.Contains(r.list, re) {
		r.list = append(r.list, re)
	}
	return nil
}

func (r *ReactionRepository) Unreact(_ context.Context, re domain.Reaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = slices.DeleteFunc(r.list, func(x domain.Reaction) bool { return x == re })
	return nil
}

func (r *ReactionRepository) ForComments(_ context.Context, comments []domain.CommentID) ([]domain.Reaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Reaction{}
	for _, re := range r.list {
		if slices.Contains(comments, re.Comment) {
			out = append(out, re)
		}
	}
	return out, nil
}
