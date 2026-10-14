// Package repotest is the contract every Webhook context repository must pass.
package repotest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

// Users and workspace the factory must create.
const (
	Ann            = "93000000-0000-4000-8000-000000000001"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ae"
)

func id(n int) string { return "94000000-0000-4000-8000-00000000000" + string(rune('0'+n)) }

var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// Repos are the three repositories of one adapter, sharing storage (deliveries belong to webhooks).
type Repos struct {
	Webhooks   domain.Repository
	Deliveries domain.DeliveryRepository
	Links      domain.LinkRepository
}

func Run(t *testing.T, r Repos) {
	ctx := tenant.With(context.Background(), tenant.Default)
	other := tenant.With(context.Background(), OtherWorkspace)
	mk := func(n int, project string, at time.Time) *domain.Webhook {
		w, err := domain.New(domain.ID(id(n)), project, "https://ci.example.com/"+project, "secret", []string{"issue.created"}, Ann, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Webhooks.Save(ctx, w); err != nil {
			t.Fatalf("save: %v", err)
		}
		return w
	}
	first := mk(1, "KYB", t0)
	mk(2, "KYB", t0.Add(time.Minute))
	mk(3, "OPS", t0)

	got, err := r.Webhooks.ByID(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("by id = %+v, %v", got, err)
	}
	if err := first.Reconfigure("https://ci.example.com/v2", []string{"*"}, false); err != nil {
		t.Fatal(err)
	}
	if err := r.Webhooks.Save(ctx, first); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err := r.Webhooks.ForProject(ctx, "KYB")
	if err != nil || len(list) != 2 || !reflect.DeepEqual(list[0], first) || list[1].ID != domain.ID(id(2)) {
		t.Fatalf("for project = %+v, %v", list, err)
	}
	if _, err := r.Webhooks.ByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := r.Webhooks.ByID(other, first.ID); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("other workspace by id: %v", err)
	}
	if list, _ := r.Webhooks.ForProject(other, "KYB"); len(list) != 0 {
		t.Fatalf("other workspace lists %v", list)
	}
	if err := r.Webhooks.Delete(other, first.ID); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("other workspace delete: %v", err)
	}

	// Deliveries: idempotent enqueue, claim across workspaces with a lease, newest-first history.
	d1 := domain.NewDelivery(id(5), tenant.Default, first.ID, "issue.created", "1/"+id(1), []byte(`{"n":1}`), t0)
	d2 := domain.NewDelivery(id(6), tenant.Default, first.ID, "issue.created", "2/"+id(1), []byte(`{"n":2}`), t0.Add(time.Second))
	for _, d := range []domain.Delivery{d1, d2, d1} {
		if err := r.Deliveries.Enqueue(ctx, d); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}
	dup := d1
	dup.ID = id(7)
	if err := r.Deliveries.Enqueue(ctx, dup); err != nil {
		t.Fatalf("enqueue duplicate key: %v", err)
	}
	now := t0.Add(time.Minute)
	claimed, err := r.Deliveries.Claim(context.Background(), now, time.Minute, 10)
	if err != nil || len(claimed) != 2 || claimed[0].ID != d1.ID || claimed[1].ID != d2.ID || string(claimed[0].Body) != `{"n":1}` ||
		claimed[0].Workspace != tenant.Default || claimed[0].Webhook != first.ID || claimed[0].Key != d1.Key {
		t.Fatalf("claim = %+v, %v", claimed, err)
	}
	if again, _ := r.Deliveries.Claim(context.Background(), now, time.Minute, 10); len(again) != 0 {
		t.Fatalf("leased deliveries claimed again: %v", again)
	}
	c := claimed[0]
	c.Failed(500, "boom", false, now)
	if err := r.Deliveries.Save(ctx, c); err != nil {
		t.Fatal(err)
	}
	d := claimed[1]
	d.Delivered(200, now)
	if err := r.Deliveries.Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	if due, _ := r.Deliveries.Claim(context.Background(), now.Add(31*time.Second), time.Minute, 10); len(due) != 1 || due[0].ID != c.ID || due[0].Attempts != 1 {
		t.Fatalf("retry due = %+v", due)
	}
	recent, err := r.Deliveries.Recent(ctx, first.ID, 1)
	if err != nil || len(recent) != 1 || recent[0].ID != d2.ID || recent[0].Status != domain.DeliveryDelivered || recent[0].LastStatus != 200 ||
		!recent[0].DeliveredAt.Equal(now) || recent[0].DeliveredAt.Location() != time.UTC {
		t.Fatalf("recent = %+v, %v", recent, err)
	}
	if all, _ := r.Deliveries.Recent(ctx, first.ID, 10); len(all) != 2 || all[1].LastError != "boom" || all[1].LastStatus != 500 || !all[1].DeliveredAt.IsZero() {
		t.Fatalf("all recent = %+v", all)
	}
	if list, _ := r.Deliveries.Recent(other, first.ID, 10); len(list) != 0 {
		t.Fatalf("other workspace sees deliveries %v", list)
	}

	if err := r.Webhooks.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Webhooks.ByID(ctx, first.ID); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if list, _ := r.Deliveries.Recent(ctx, first.ID, 10); len(list) != 0 {
		t.Fatalf("deliveries outlive their webhook: %v", list)
	}
	if due, _ := r.Deliveries.Claim(context.Background(), now.Add(time.Hour), time.Minute, 10); len(due) != 0 {
		t.Fatalf("deliveries of a deleted webhook claimed: %v", due)
	}
	if err := r.Webhooks.Delete(ctx, first.ID); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("delete twice: %v", err)
	}

	// Development links: upsert by (issue, kind, repo, ref), most recently updated first.
	link := func(issue string, kind domain.LinkKind, ref, title, state string, at time.Time) domain.Link {
		l, err := domain.NewLink(issue, kind, "o/r", ref, "https://github.com/o/r/"+ref, title, state, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Links.Save(ctx, l); err != nil {
			t.Fatalf("save link: %v", err)
		}
		return l
	}
	link("KYB-1", domain.LinkCommit, "abc", "KYB-1 fix", "", t0)
	link("KYB-1", domain.LinkPullRequest, "7", "KYB-1 draft", "open", t0.Add(time.Minute))
	link("KYB-2", domain.LinkCommit, "abc", "KYB-2 too", "", t0)
	merged := link("KYB-1", domain.LinkPullRequest, "7", "KYB-1 ready", "merged", t0.Add(2*time.Minute))
	links, err := r.Links.ForIssue(ctx, "KYB-1")
	if err != nil || len(links) != 2 || links[0] != merged || links[1].Ref != "abc" || links[1].UpdatedAt.Location() != time.UTC {
		t.Fatalf("links = %+v, %v", links, err)
	}
	if list, _ := r.Links.ForIssue(other, "KYB-1"); len(list) != 0 {
		t.Fatalf("other workspace sees links %v", list)
	}
}
