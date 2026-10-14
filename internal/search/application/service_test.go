package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/search/application"
	"github.com/bakhod1r/kyber/internal/search/domain"
	"github.com/bakhod1r/kyber/internal/search/infrastructure/memory"
)

type fakes struct {
	projects map[string][]string             // actor → browsable projects
	issues   map[string][]domain.Issue       // project → facts
	sprints  map[string][]application.Sprint // project → sprints
	versions map[string][]application.NamedID
	fail     string   // port to fail: projects, issues, sprints, versions, users
	read     []string // projects whose facts were read, with the narrow passed
}

var boom = errors.New("boom")

func (f *fakes) Browsable(_ context.Context, actor string) ([]string, error) {
	if f.fail == "projects" {
		return nil, boom
	}
	return f.projects[actor], nil
}
func (f *fakes) Facts(_ context.Context, _, project string, n domain.Narrow) ([]domain.Issue, error) {
	if f.fail == "issues" {
		return nil, boom
	}
	f.read = append(f.read, fmt.Sprintf("%s%v", project, n.Types))
	return f.issues[project], nil
}
func (f *fakes) CustomFields(_ context.Context, project string) (map[string]domain.CustomType, error) {
	if f.fail == "fields" {
		return nil, boom
	}
	if project == "KYB" {
		return map[string]domain.CustomType{"customer": domain.CustomText}, nil
	}
	return map[string]domain.CustomType{"customer": domain.CustomNumber}, nil // first project's type wins
}

func (f *fakes) Sprints(_ context.Context, project string) ([]application.Sprint, error) {
	if f.fail == "sprints" {
		return nil, boom
	}
	return f.sprints[project], nil
}
func (f *fakes) Versions(_ context.Context, project string) ([]application.NamedID, error) {
	if f.fail == "versions" {
		return nil, boom
	}
	return f.versions[project], nil
}
func (f *fakes) UserIDByEmail(_ context.Context, email string) (string, bool, error) {
	if f.fail == "users" {
		return "", false, boom
	}
	id, ok := map[string]string{"bob@x.uz": "u-bob"}[strings.ToLower(email)]
	return id, ok, nil
}

func setup() (*application.Service, *fakes) {
	f := &fakes{
		projects: map[string][]string{"u-ali": {"KYB", "OPS"}, "u-bob": {"KYB"}},
		issues: map[string][]domain.Issue{
			"KYB": {
				{Key: "KYB-1", Project: "KYB", Number: 1, Summary: "Login", Assignee: "u-bob", Sprints: []string{"s-1"}, FixVersions: []string{"v-1"},
					Custom: map[string]string{"customer": "Acme"}},
				{Key: "KYB-2", Project: "KYB", Number: 2, Summary: "Docs", Assignee: "u-ali", Sprints: []string{"s-2"}},
			},
			"OPS": {{Key: "OPS-1", Project: "OPS", Number: 1, Summary: "Keys", Assignee: "u-ali"}},
		},
		sprints:  map[string][]application.Sprint{"KYB": {{NamedID: application.NamedID{ID: "s-1", Name: "Sprint 1"}, Open: true}, {NamedID: application.NamedID{ID: "s-2", Name: "Sprint 0"}}}},
		versions: map[string][]application.NamedID{"KYB": {{ID: "v-1", Name: "1.0"}}},
	}
	n := 0
	return application.NewService(application.Deps{Projects: f, Issues: f, Users: f, Planning: f, Filters: memory.NewFilterRepository(),
		NewID: func() string { n++; return fmt.Sprintf("f-%d", n) }}), f
}

func keys(r application.Result) string {
	var out []string
	for _, is := range r.Issues {
		out = append(out, is.Key)
	}
	return fmt.Sprintf("%s/%d", strings.Join(out, ","), r.Total)
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	s, f := setup()
	for jql, want := range map[string]string{
		"":                         "KYB-1,KYB-2,OPS-1/3",
		"assignee = currentUser()": "KYB-2,OPS-1/2",
		`assignee = "BOB@x.uz" and assignee = "bob@x.uz"`: "KYB-1/1",
		"sprint in openSprints()":                         "KYB-1/1",
		`sprint = "sprint 0"`:                             "KYB-2/1",
		`fixVersion = "1.0"`:                              "KYB-1/1",
		`customer ~ acme`:                                 "KYB-1/1",
	} {
		r, err := s.Search(ctx, "u-ali", jql, 0)
		if err != nil || keys(r) != want {
			t.Errorf("%q = %s %v, want %s", jql, keys(r), err, want)
		}
	}
	// Only browsable projects are searched; limits cap the page, not the total.
	if r, _ := s.Search(ctx, "u-bob", "", 1); keys(r) != "KYB-1/2" {
		t.Fatalf("bob = %s", keys(r))
	}
	if _, err := s.Search(ctx, "u-ali", "project ==", 0); !errors.Is(err, domain.ErrInvalidJQL) {
		t.Fatalf("bad jql: %v", err)
	}
	for _, port := range []string{"projects", "fields", "issues", "sprints", "versions", "users"} {
		f.fail = port
		if _, err := s.Search(ctx, "u-ali", `assignee = "x@x.uz"`, 0); !errors.Is(err, boom) {
			t.Errorf("%s failure: %v", port, err)
		}
	}
}

// JQL pushdown: a query naming projects reads only those, and passes its narrow to the store.
func TestSearchPushdown(t *testing.T) {
	s, f := setup()
	f.read = nil
	r, err := s.Search(context.Background(), "u-ali", "project = kyb AND type in (bug, story)", 0)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(f.read) != "[KYB[bug story]]" {
		t.Fatalf("read = %v (result %s)", f.read, keys(r))
	}
}

func TestFilters(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if _, err := s.SaveFilter(ctx, "u-ali", "x", "project ==", false); !errors.Is(err, domain.ErrInvalidJQL) {
		t.Fatalf("bad jql: %v", err)
	}
	mine, err := s.SaveFilter(ctx, "u-ali", "My open", "assignee = currentUser()", false)
	if err != nil {
		t.Fatal(err)
	}
	shared, _ := s.SaveFilter(ctx, "u-ali", "Team", "project = KYB", true)
	if list, _ := s.Filters(ctx, "u-bob"); len(list) != 1 || list[0].Name != "Team" {
		t.Fatalf("bob sees %+v", list)
	}
	if _, err := s.UpdateFilter(ctx, "u-bob", string(shared.ID), 1, "Mine now", "", true); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("bob edits a shared filter: %v", err)
	}
	if err := s.DeleteFilter(ctx, "u-bob", string(mine.ID)); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("bob deletes a private filter: %v", err)
	}
	if _, err := s.UpdateFilter(ctx, "u-ali", string(mine.ID), 9, "x", "", false); !errors.Is(err, domain.ErrFilterConflict) {
		t.Fatalf("stale: %v", err)
	}
	if _, err := s.UpdateFilter(ctx, "u-ali", string(mine.ID), 1, "", "", false); !errors.Is(err, domain.ErrInvalidFilter) {
		t.Fatalf("invalid: %v", err)
	}
	f, err := s.UpdateFilter(ctx, "u-ali", string(mine.ID), 1, "Renamed", "type = bug", true)
	if err != nil || f.Name != "Renamed" || f.Version != 2 {
		t.Fatalf("update: %+v %v", f, err)
	}
	if err := s.DeleteFilter(ctx, "u-ali", string(mine.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateFilter(ctx, "u-ali", "f-404", 1, "x", "", false); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
