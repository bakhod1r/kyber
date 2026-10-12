package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type GroupRepository struct {
	mu     sync.Mutex
	tokens map[string]domain.GroupToken
	chats  map[string]domain.ProjectChat // workspace/project/chat
}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{tokens: map[string]domain.GroupToken{}, chats: map[string]domain.ProjectChat{}}
}

func (r *GroupRepository) CreateGroupToken(_ context.Context, t domain.GroupToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[t.Hash] = t
	return nil
}

func (r *GroupRepository) ConsumeGroupToken(_ context.Context, hash string, now time.Time) (domain.GroupToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[hash]
	delete(r.tokens, hash)
	if !ok || now.After(t.ExpiresAt) {
		return domain.GroupToken{}, domain.ErrGroupTokenInvalid
	}
	return t, nil
}

func chatKey(ws, project, chat string) string { return ws + "/" + project + "/" + chat }

func (r *GroupRepository) LinkChat(_ context.Context, c domain.ProjectChat) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := chatKey(c.Workspace, c.Project, c.ChatID)
	if old, ok := r.chats[k]; ok {
		old.Title = c.Title
		r.chats[k] = old
		return nil
	}
	r.chats[k] = c
	return nil
}

func (r *GroupRepository) UnlinkChat(ctx context.Context, project, chatID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := chatKey(tenant.Workspace(ctx), project, chatID)
	if _, ok := r.chats[k]; !ok {
		return domain.ErrChatNotLinked
	}
	delete(r.chats, k)
	return nil
}

func (r *GroupRepository) ChatsOf(ctx context.Context, project string) ([]domain.ProjectChat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.ProjectChat{}
	for _, c := range r.chats {
		if c.Workspace == tenant.Workspace(ctx) && c.Project == project {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChatID < out[j].ChatID })
	return out, nil
}
