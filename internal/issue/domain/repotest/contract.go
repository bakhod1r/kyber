// Package repotest is the contract every domain.Repository adapter must satisfy.
package repotest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Assignee is a user ID adapters must accept as an assignee (the Postgres
// test factory inserts this user so the foreign key holds).
const Assignee = "60000000-0000-4000-8000-000000000001"

// SprintA is a sprint ID adapters must accept (the Postgres factory inserts it).
const SprintA = "70000000-0000-4000-8000-000000000001"

// VersionA and VersionB are KYB versions adapters must accept (the Postgres factory inserts them).
const (
	VersionA = "71000000-0000-4000-8000-000000000001"
	VersionB = "71000000-0000-4000-8000-000000000002"
)

func keys(list []*domain.Issue) string {
	out := make([]string, len(list))
	for i, is := range list {
		out[i] = is.Key().String()
	}
	return strings.Join(out, ",")
}

// Factory returns an empty repository plus a function reading stored outbox event names.
type Factory func(t *testing.T) (domain.Repository, func() []string)

func Run(t *testing.T, newRepo Factory) {
	ctx := context.Background()
	mk := func(t *testing.T, n int, title string) *domain.Issue {
		t.Helper()
		key, _ := domain.NewIssueKey("KYB", n)
		is, err := domain.NewIssue(domain.IssueID(uuidFor(n)), key, title, domain.TypeTask, domain.UserID(Assignee), domain.DefaultWorkflow())
		if err != nil {
			t.Fatal(err)
		}
		is.Rerank(domain.Rank(fmt.Sprintf("b%02d", n))) // rank order == number order unless a test reranks
		return is
	}

	t.Run("save new then load", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		if err := repo.Save(ctx, is, is.PullEvents()); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ByKey(ctx, is.Key())
		if err != nil {
			t.Fatal(err)
		}
		if got.ID() != is.ID() || got.Title() != "First" || got.Status() != domain.StatusTodo || got.Version() != 1 {
			t.Fatalf("loaded %+v", got)
		}
		if ev := outbox(); len(ev) != 1 || ev[0] != "issue.created" {
			t.Fatalf("outbox = %v", ev)
		}
	})

	t.Run("parent links persist and filter children", func(t *testing.T) {
		repo, _ := newRepo(t)
		key1, _ := domain.NewIssueKey("KYB", 1)
		epic, _ := domain.NewIssue(domain.IssueID(uuidFor(1)), key1, "Epic", domain.TypeEpic, domain.UserID(Assignee), domain.DefaultWorkflow())
		_ = repo.Save(ctx, epic, nil)
		a, b := mk(t, 2, "A"), mk(t, 3, "B")
		_ = repo.Save(ctx, a, nil)
		_ = repo.Save(ctx, b, nil)
		if err := a.SetParent(epic); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, a, nil); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, a.Key())
		if got.Parent() != epic.ID() || got.ParentKey() != key1 {
			t.Fatalf("parent = %q %v", got.Parent(), got.ParentKey())
		}
		parent := epic.ID()
		if kids, _ := repo.ListByProject(ctx, "KYB", domain.ListFilter{Parent: &parent}); keys(kids) != "KYB-2" {
			t.Fatalf("children = %s", keys(kids))
		}
		_ = got.SetParent(nil)
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, a.Key()); again.Parent() != "" {
			t.Fatal("parent not cleared")
		}
	})

	t.Run("security level persists", func(t *testing.T) {
		repo, _ := newRepo(t)
		is := mk(t, 1, "Secret")
		is.Classify("confidential")
		if err := repo.Save(ctx, is, nil); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, is.Key())
		if got.SecurityLevel() != "confidential" {
			t.Fatalf("level = %q", got.SecurityLevel())
		}
		got.Classify("")
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); again.SecurityLevel() != "" {
			t.Fatal("level not cleared")
		}
		// The resolution is persisted.
		res, _ := repo.ByKey(ctx, is.Key())
		if err := res.TransitionResolved(domain.StatusDone, domain.DefaultWorkflow(), domain.ResolutionDuplicate); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, res, nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.ByKey(ctx, is.Key()); got.Resolution() != domain.ResolutionDuplicate {
			t.Fatalf("resolution = %q", got.Resolution())
		}
		// The schedule is persisted and can be cleared.
		sch, _ := repo.ByKey(ctx, is.Key())
		if err := sch.Plan("2026-10-13", "2026-10-31"); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, sch, nil); err != nil {
			t.Fatal(err)
		}
		got, _ = repo.ByKey(ctx, is.Key())
		if s, d := got.Schedule(); s != "2026-10-13" || d != "2026-10-31" {
			t.Fatalf("schedule = %q %q", s, d)
		}
		_ = got.Plan("", "")
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); again.Snapshot().DueDate != "" {
			t.Fatal("schedule not cleared")
		}
	})

	t.Run("fix versions persist and filter", func(t *testing.T) {
		repo, _ := newRepo(t)
		a, b := mk(t, 1, "In both"), mk(t, 2, "In A")
		a.PlanForVersions([]domain.VersionID{VersionB, VersionA})
		b.PlanForVersions([]domain.VersionID{VersionA})
		for _, is := range []*domain.Issue{a, b, mk(t, 3, "None")} {
			if err := repo.Save(ctx, is, nil); err != nil {
				t.Fatal(err)
			}
		}
		got, _ := repo.ByKey(ctx, a.Key())
		if fv := got.FixVersions(); len(fv) != 2 || fv[0] != VersionA || fv[1] != VersionB {
			t.Fatalf("fix versions = %v", fv)
		}
		va, vb, none := domain.VersionID(VersionA), domain.VersionID(VersionB), domain.VersionID("nope")
		for v, want := range map[*domain.VersionID]string{&va: "KYB-1,KYB-2", &vb: "KYB-1", &none: ""} {
			list, err := repo.ListByProject(ctx, "KYB", domain.ListFilter{FixVersion: v})
			if err != nil || keys(list) != want {
				t.Errorf("filter %s = %s %v", *v, keys(list), err)
			}
		}
		got.MoveToVersion(VersionB, "")
		if err := repo.Save(ctx, got, nil); err != nil {
			t.Fatal(err)
		}
		if again, _ := repo.ByKey(ctx, a.Key()); len(again.FixVersions()) != 1 || again.FixVersions()[0] != VersionA {
			t.Fatalf("after removal = %v", again.FixVersions())
		}
	})

	t.Run("labels persist and filter", func(t *testing.T) {
		repo, _ := newRepo(t)
		a, b := mk(t, 1, "Both"), mk(t, 2, "UI")
		if err := a.Relabel([]string{"ui", "backend"}); err != nil {
			t.Fatal(err)
		}
		_ = b.Relabel([]string{"ui"})
		for _, is := range []*domain.Issue{a, b, mk(t, 3, "None")} {
			if err := repo.Save(ctx, is, nil); err != nil {
				t.Fatal(err)
			}
		}
		got, _ := repo.ByKey(ctx, a.Key())
		if l := got.Labels(); len(l) != 2 || l[0] != "backend" || l[1] != "ui" {
			t.Fatalf("labels = %v", l)
		}
		ui, be, none := domain.Label("ui"), domain.Label("backend"), domain.Label("nope")
		for l, want := range map[*domain.Label]string{&ui: "KYB-1,KYB-2", &be: "KYB-1", &none: ""} {
			list, err := repo.ListByProject(ctx, "KYB", domain.ListFilter{Label: l})
			if err != nil || keys(list) != want {
				t.Errorf("filter %s = %s %v", *l, keys(list), err)
			}
		}
		_ = got.Relabel(nil)
		if err := repo.Save(ctx, got, nil); err != nil {
			t.Fatal(err)
		}
		if again, _ := repo.ByKey(ctx, a.Key()); len(again.Labels()) != 0 {
			t.Fatalf("after clearing = %v", again.Labels())
		}
	})

	t.Run("time tracking persists", func(t *testing.T) {
		repo, _ := newRepo(t)
		is := mk(t, 1, "Timed")
		is.EstimateTime(8 * 3600)
		_, _ = is.LogWork("w", Assignee, time.Now(), 1800, "")
		if err := repo.Save(ctx, is, nil); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, is.Key())
		if got.TimeTracking() != (domain.TimeTracking{Original: 8 * 3600, Remaining: 8*3600 - 1800, Spent: 1800}) {
			t.Fatalf("time = %+v", got.TimeTracking())
		}
		got.EstimateRemaining(60)
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); again.TimeTracking().Remaining != 60 {
			t.Fatalf("updated = %+v", again.TimeTracking())
		}
	})

	t.Run("custom field values persist", func(t *testing.T) {
		repo, _ := newRepo(t)
		is := mk(t, 1, "Fields")
		tier, _ := domain.NewCustomField("f-tier", "KYB", "Tier", domain.FieldSelect, []string{"Gold"})
		_ = is.SetField(tier, "gold")
		if err := repo.Save(ctx, is, nil); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, is.Key())
		if got.Field("f-tier") != "Gold" {
			t.Fatalf("fields = %v", got.Fields())
		}
		_ = got.SetField(tier, "")
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); len(again.Fields()) != 0 {
			t.Fatalf("cleared = %v", again.Fields())
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, _ := newRepo(t)
		key, _ := domain.NewIssueKey("KYB", 42)
		if _, err := repo.ByKey(ctx, key); !errors.Is(err, domain.ErrIssueNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("update bumps version and stores events", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		_ = repo.Save(ctx, is, is.PullEvents())
		loaded, _ := repo.ByKey(ctx, is.Key())
		if err := loaded.Transition(domain.StatusInProgress, domain.DefaultWorkflow()); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, loaded, loaded.PullEvents()); err != nil {
			t.Fatal(err)
		}
		again, _ := repo.ByKey(ctx, is.Key())
		if again.Version() != 2 || again.Status() != domain.StatusInProgress {
			t.Fatalf("loaded %+v", again)
		}
		if ev := outbox(); len(ev) != 2 || ev[1] != "issue.transitioned" {
			t.Fatalf("outbox = %v", ev)
		}
	})

	t.Run("stale version is rejected atomically", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		_ = repo.Save(ctx, is, is.PullEvents())
		a, _ := repo.ByKey(ctx, is.Key())
		b, _ := repo.ByKey(ctx, is.Key())
		_ = a.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		if err := repo.Save(ctx, a, a.PullEvents()); err != nil {
			t.Fatal(err)
		}
		_ = b.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		if err := repo.Save(ctx, b, b.PullEvents()); !errors.Is(err, domain.ErrConcurrentModification) {
			t.Fatalf("err = %v, want ErrConcurrentModification", err)
		}
		if ev := outbox(); len(ev) != 2 {
			t.Fatalf("failed save must not write events: %v", ev)
		}
	})

	t.Run("duplicate new key is rejected", func(t *testing.T) {
		repo, _ := newRepo(t)
		a := mk(t, 1, "A")
		_ = repo.Save(ctx, a, nil)
		b := mk(t, 2, "B")
		bKeyDup := domain.Rehydrate(domain.Snapshot{ID: b.ID(), Key: a.Key(), Title: "B", Type: domain.TypeTask, Status: domain.StatusTodo, Rank: "a0"})
		if err := repo.Save(ctx, bKeyDup, nil); !errors.Is(err, domain.ErrConcurrentModification) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("details round-trip", func(t *testing.T) {
		repo, _ := newRepo(t)
		key1, _ := domain.NewIssueKey("KYB", 1)
		is, _ := domain.NewIssue(domain.IssueID(uuidFor(1)), key1, "First", domain.TypeTask, domain.UserID(Assignee), domain.DefaultWorkflow())
		is.Rerank("b01")
		_ = repo.Save(ctx, is, nil)
		loaded, _ := repo.ByKey(ctx, is.Key())
		if loaded.Reporter() != domain.UserID(Assignee) {
			t.Fatalf("reporter = %q", loaded.Reporter())
		}
		desc, prio := "Line 1\nLine 2 ✓", domain.PriorityHighest
		if err := loaded.Edit(domain.Changes{Description: &desc, Priority: &prio}); err != nil {
			t.Fatal(err)
		}
		loaded.Assign(domain.UserID(Assignee), "u-actor")
		pts := domain.Points(25)
		loaded.SetEstimate(&pts)
		if err := repo.Save(ctx, loaded, loaded.PullEvents()); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, is.Key())
		if e, ok := got.Estimate(); !ok || e != 25 {
			t.Fatalf("estimate = %v %v", e, ok)
		}
		if got.Description() != desc || got.Priority() != prio || got.Assignee() != domain.UserID(Assignee) {
			t.Fatalf("loaded %q %q %q", got.Description(), got.Priority(), got.Assignee())
		}
		got.Assign("", "u-actor")
		got.SetEstimate(nil)
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); again.Assignee() != "" {
			t.Fatalf("unassign not persisted: %q", again.Assignee())
		}
		if again, _ := repo.ByKey(ctx, is.Key()); func() bool { _, ok := again.Estimate(); return ok }() {
			t.Fatalf("unassign not persisted: %q", again.Assignee())
		}
	})

	t.Run("rank order, neighbours and sprint filter", func(t *testing.T) {
		repo, _ := newRepo(t)
		for _, n := range []int{1, 2, 3} {
			_ = repo.Save(ctx, mk(t, n, "t"), nil)
		}
		k1, _ := domain.NewIssueKey("KYB", 1)
		one, _ := repo.ByKey(ctx, k1)
		one.Rerank("b99") // move KYB-1 to the bottom
		one.MoveToSprint(domain.SprintID(SprintA))
		if err := repo.Save(ctx, one, nil); err != nil {
			t.Fatal(err)
		}
		all, _ := repo.ListByProject(ctx, "KYB", domain.ListFilter{})
		if got := keys(all); got != "KYB-2,KYB-3,KYB-1" {
			t.Fatalf("rank order = %s", got)
		}
		backlog := domain.SprintID("")
		inBacklog, _ := repo.ListByProject(ctx, "KYB", domain.ListFilter{Sprint: &backlog})
		sprint := domain.SprintID(SprintA)
		inSprint, _ := repo.ListByProject(ctx, "KYB", domain.ListFilter{Sprint: &sprint})
		if keys(inBacklog) != "KYB-2,KYB-3" || keys(inSprint) != "KYB-1" {
			t.Fatalf("backlog = %s, sprint = %s", keys(inBacklog), keys(inSprint))
		}

		last, err := repo.LastRank(ctx, "KYB")
		if err != nil || last != "b99" {
			t.Fatalf("LastRank = %q, %v", last, err)
		}
		if none, _ := repo.LastRank(ctx, "NONE"); none != "" {
			t.Fatalf("LastRank(empty project) = %q", none)
		}
		if next, _ := repo.NextRank(ctx, "KYB", "b02"); next != "b03" {
			t.Fatalf("NextRank(b02) = %q", next)
		}
		if next, _ := repo.NextRank(ctx, "KYB", "b99"); next != "" {
			t.Fatalf("NextRank(last) = %q", next)
		}
		if prev, _ := repo.PrevRank(ctx, "KYB", "b03"); prev != "b02" {
			t.Fatalf("PrevRank(b03) = %q", prev)
		}
		if prev, _ := repo.PrevRank(ctx, "KYB", "b02"); prev != "" {
			t.Fatalf("PrevRank(first) = %q", prev)
		}
	})

	t.Run("list by project ordered and filtered", func(t *testing.T) {
		repo, _ := newRepo(t)
		for _, n := range []int{3, 1, 2} {
			is := mk(t, n, "t")
			_ = repo.Save(ctx, is, nil)
		}
		other, _ := domain.NewIssueKey("OPS", 1)
		_ = repo.Save(ctx, domain.Rehydrate(domain.Snapshot{ID: domain.IssueID(uuidFor(99)), Key: other, Title: "o", Type: domain.TypeTask, Status: domain.StatusTodo, Rank: "a0"}), nil)
		k2, _ := domain.NewIssueKey("KYB", 2)
		two, _ := repo.ByKey(ctx, k2)
		_ = two.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		_ = repo.Save(ctx, two, nil)

		all, err := repo.ListByProject(ctx, "KYB", domain.ListFilter{})
		if err != nil || len(all) != 3 || all[0].Key().Number() != 1 || all[2].Key().Number() != 3 {
			t.Fatalf("all = %v, %v", all, err)
		}
		st := domain.StatusInProgress
		f, _ := repo.ListByProject(ctx, "KYB", domain.ListFilter{Status: &st})
		if len(f) != 1 || f[0].Key().Number() != 2 {
			t.Fatalf("filtered = %v", f)
		}
	})
}

func uuidFor(n int) string {
	const base = "00000000-0000-4000-8000-0000000000"
	return base + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

// CommentFactory returns an empty comment repository, a saved issue to attach comments to,
// and a function reading outbox event names.
type CommentFactory func(t *testing.T) (domain.CommentRepository, *domain.Issue, func() []string)

func RunComments(t *testing.T, newRepo CommentFactory) {
	ctx := context.Background()
	t.Run("add and list oldest first", func(t *testing.T) {
		repo, is, outbox := newRepo(t)
		base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		for i, body := range []string{"second", "first", "third"} {
			at := base.Add(time.Duration([]int{2, 1, 3}[i]) * time.Minute)
			c, err := domain.NewComment(domain.CommentID(uuidFor(10+i)), is.ID(), is.Key(), domain.UserID(Assignee), body, at)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Add(ctx, c, c.PullEvents()); err != nil {
				t.Fatal(err)
			}
		}
		list, err := repo.ListByIssue(ctx, is.ID())
		if err != nil || len(list) != 3 {
			t.Fatalf("list = %v, %v", list, err)
		}
		if list[0].Body() != "first" || list[2].Body() != "third" || !list[0].CreatedAt().Equal(base.Add(time.Minute)) {
			t.Fatalf("order/time wrong: %q %q %v", list[0].Body(), list[2].Body(), list[0].CreatedAt())
		}
		if list[0].Author() != domain.UserID(Assignee) || list[0].IssueID() != is.ID() {
			t.Fatalf("fields wrong: %+v", list[0])
		}
		if ev := outbox(); len(ev) != 3 || ev[0] != "comment.added" {
			t.Fatalf("outbox = %v", ev)
		}
	})
	t.Run("issue key of a comment", func(t *testing.T) {
		repo, is, _ := newRepo(t)
		c, _ := domain.NewComment(domain.CommentID(uuidFor(20)), is.ID(), is.Key(), domain.UserID(Assignee), "hi", time.Now())
		if err := repo.Add(ctx, c, nil); err != nil {
			t.Fatal(err)
		}
		if k, err := repo.IssueKeyOf(ctx, c.ID()); err != nil || k != is.Key() {
			t.Fatalf("key = %v, %v", k, err)
		}
		for _, id := range []string{uuidFor(21), "not-a-uuid"} {
			if _, err := repo.IssueKeyOf(ctx, domain.CommentID(id)); !errors.Is(err, domain.ErrCommentNotFound) {
				t.Fatalf("unknown %q: %v", id, err)
			}
		}
	})
	t.Run("other issue has no comments", func(t *testing.T) {
		repo, _, _ := newRepo(t)
		list, err := repo.ListByIssue(ctx, domain.IssueID(uuidFor(98)))
		if err != nil || len(list) != 0 {
			t.Fatalf("list = %v, %v", list, err)
		}
	})
}
