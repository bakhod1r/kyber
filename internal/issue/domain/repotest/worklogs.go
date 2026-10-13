package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// WorklogFactory returns an empty worklog repository and two saved issues (users Assignee exists).
type WorklogFactory func(t *testing.T) (domain.WorklogRepository, [2]*domain.Issue)

func RunWorklogs(t *testing.T, newRepo WorklogFactory) {
	ctx := context.Background()
	repo, is := newRepo(t)
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	w1 := domain.Worklog{ID: domain.WorklogID(uuidFor(201)), Issue: is[0].ID(), Key: is[0].Key(), Author: Assignee, Started: at.Add(time.Hour), Seconds: 1800, Comment: "later"}
	w2 := domain.Worklog{ID: domain.WorklogID(uuidFor(202)), Issue: is[0].ID(), Key: is[0].Key(), Author: Assignee, Started: at, Seconds: 3600, Comment: "first"}
	w3 := domain.Worklog{ID: domain.WorklogID(uuidFor(203)), Issue: is[1].ID(), Key: is[1].Key(), Author: Assignee, Started: at, Seconds: 60}
	for _, w := range []domain.Worklog{w1, w2, w3, w1} { // w1 twice: redelivery
		if err := repo.Add(ctx, w); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.ForIssue(ctx, is[0].ID())
	if err != nil || len(list) != 2 || list[0].ID != w2.ID || list[1].ID != w1.ID {
		t.Fatalf("worklogs = %+v %v (oldest first)", list, err)
	}
	if list[0].Comment != "first" || list[0].Seconds != 3600 || !list[0].Started.Equal(at) || list[0].Author != Assignee || list[0].Key != is[0].Key() {
		t.Fatalf("values = %+v", list[0])
	}
	got, err := repo.ByID(ctx, w3.ID)
	if err != nil || got.Issue != is[1].ID() || got.Seconds != 60 {
		t.Fatalf("by id = %+v %v", got, err)
	}
	for _, id := range []domain.WorklogID{w1.ID, w1.ID} {
		if err := repo.Remove(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.ByID(ctx, w1.ID); !errors.Is(err, domain.ErrWorklogNotFound) {
		t.Fatalf("removed: %v", err)
	}
	if _, err := repo.ByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrWorklogNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if list, _ := repo.ForIssue(ctx, is[0].ID()); len(list) != 1 {
		t.Fatalf("after remove = %+v", list)
	}
}
