package repotest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// RunGroups needs projects KYB and OPS in the default workspace and user Alice.
func RunGroups(t *testing.T, newRepo func(t *testing.T) domain.GroupRepository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	t.Run("tokens work once and expire", func(t *testing.T) {
		repo := newRepo(t)
		raw, tok := domain.NewGroupToken(tenant.Default, "KYB", Alice, t0)
		if err := repo.CreateGroupToken(ctx, tok); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ConsumeGroupToken(ctx, domain.HashGroupToken(raw), t0.Add(time.Minute))
		if err != nil || got.Project != "KYB" || got.Workspace != tenant.Default || got.User != Alice {
			t.Fatalf("consume = %+v, %v", got, err)
		}
		if _, err := repo.ConsumeGroupToken(ctx, domain.HashGroupToken(raw), t0); !errors.Is(err, domain.ErrGroupTokenInvalid) {
			t.Fatalf("reuse err = %v", err)
		}
		raw, tok = domain.NewGroupToken(tenant.Default, "KYB", Alice, t0)
		_ = repo.CreateGroupToken(ctx, tok)
		if _, err := repo.ConsumeGroupToken(ctx, domain.HashGroupToken(raw), t0.Add(domain.GroupTokenTTL+time.Second)); !errors.Is(err, domain.ErrGroupTokenInvalid) {
			t.Fatalf("expired err = %v", err)
		}
		raw, tok = domain.NewGroupToken(tenant.Default, "KYB", Alice, t0)
		_ = repo.CreateGroupToken(ctx, tok)
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := repo.ConsumeGroupToken(ctx, domain.HashGroupToken(raw), t0); err == nil {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("token used %d times", wins)
		}
	})
	t.Run("link, list, relink, unlink", func(t *testing.T) {
		repo := newRepo(t)
		chat := func(project, id, title string) domain.ProjectChat {
			return domain.ProjectChat{Workspace: tenant.Default, Project: project, ChatID: id, Title: title, LinkedBy: Alice, LinkedAt: t0}
		}
		for _, c := range []domain.ProjectChat{chat("KYB", "-1001", "Devs"), chat("KYB", "-1002", "QA"), chat("OPS", "-1001", "Devs"), chat("KYB", "-1001", "Devs (renamed)")} {
			if err := repo.LinkChat(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.ChatsOf(ctx, "KYB")
		if err != nil || len(got) != 2 || got[0].ChatID != "-1001" || got[0].Title != "Devs (renamed)" || got[1].ChatID != "-1002" || got[0].LinkedBy != Alice {
			t.Fatalf("KYB chats = %+v, %v", got, err)
		}
		if err := repo.UnlinkChat(ctx, "KYB", "-1001"); err != nil {
			t.Fatal(err)
		}
		if err := repo.UnlinkChat(ctx, "KYB", "-1001"); !errors.Is(err, domain.ErrChatNotLinked) {
			t.Fatalf("second unlink err = %v", err)
		}
		if ops, _ := repo.ChatsOf(ctx, "OPS"); len(ops) != 1 {
			t.Fatalf("other project affected: %+v", ops)
		}
		other := tenant.With(ctx, "00000000-0000-4000-8000-0000000000aa")
		if leaked, _ := repo.ChatsOf(other, "KYB"); len(leaked) != 0 {
			t.Fatalf("leaked across workspaces: %+v", leaked)
		}
		if err := repo.UnlinkChat(other, "KYB", "-1002"); !errors.Is(err, domain.ErrChatNotLinked) {
			t.Fatalf("unlinked across workspaces: %v", err)
		}
	})
}
