package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/search/domain"
)

// facts is a tiny issue set the evaluator runs over.
var facts = []domain.Issue{
	{Key: "KYB-1", Project: "KYB", Number: 1, Summary: "Login page crashes", Description: "on Safari", Type: "bug", Status: "todo", StatusName: "To Do", Category: "todo",
		Priority: "high", Assignee: "u-ali", Reporter: "u-bob", Points: ptr(3)},
	{Key: "KYB-2", Project: "KYB", Number: 2, Summary: "Sign in with Google", Type: "story", Status: "in_progress", StatusName: "In Progress", Category: "in_progress",
		Priority: "medium", Assignee: "u-bob", Reporter: "u-ali", Sprints: []string{"s-1"}, FixVersions: []string{"v-1"}, Points: ptr(5)},
	{Key: "KYB-3", Project: "KYB", Number: 3, Summary: "Docs", Type: "task", Status: "done", StatusName: "Done", Category: "done",
		Priority: "low", Reporter: "u-ali", Resolution: "wont_do", Parent: "KYB-2"},
	{Key: "OPS-1", Project: "OPS", Number: 1, Summary: "Rotate keys", Type: "task", Status: "todo", StatusName: "To Do", Category: "todo",
		Priority: "highest", Assignee: "u-ali", Reporter: "u-ali"},
}

func ptr(f float64) *float64 { return &f }

// resolver maps names to ids like the application layer does.
type resolver struct{}

func (resolver) User(name string) (string, bool) {
	id, ok := map[string]string{"ali@x.uz": "u-ali", "bob@x.uz": "u-bob"}[strings.ToLower(name)]
	return id, ok
}
func (resolver) CurrentUser() string { return "u-ali" }
func (resolver) Sprints(name string) []string {
	return map[string][]string{"sprint 1": {"s-1"}}[strings.ToLower(name)]
}
func (resolver) OpenSprints() []string { return []string{"s-1"} }
func (resolver) Versions(name string) []string {
	return map[string][]string{"1.0": {"v-1"}}[strings.ToLower(name)]
}

func run(t *testing.T, jql string) string {
	t.Helper()
	q, err := domain.Parse(jql)
	if err != nil {
		t.Fatalf("%q: %v", jql, err)
	}
	out, err := q.Run(facts, resolver{})
	if err != nil {
		t.Fatalf("%q: %v", jql, err)
	}
	keys := make([]string, len(out))
	for i, is := range out {
		keys[i] = is.Key
	}
	return strings.Join(keys, ",")
}

func TestSearch(t *testing.T) {
	cases := map[string]string{
		``:                                                   "KYB-1,KYB-2,KYB-3,OPS-1",
		`project = KYB`:                                      "KYB-1,KYB-2,KYB-3",
		`project in (kyb, OPS) and type = task`:              "KYB-3,OPS-1",
		`issuetype != bug AND project = "KYB"`:               "KYB-2,KYB-3",
		`status = "In Progress"`:                             "KYB-2",
		`status = done or status = todo`:                     "KYB-1,KYB-3,OPS-1",
		`statusCategory = "To Do"`:                           "KYB-1,OPS-1",
		`statusCategory != done`:                             "KYB-1,KYB-2,OPS-1",
		`priority in (high, highest)`:                        "KYB-1,OPS-1",
		`assignee = currentUser()`:                           "KYB-1,OPS-1",
		`assignee = "bob@x.uz"`:                              "KYB-2",
		`assignee is EMPTY`:                                  "KYB-3",
		`assignee is not empty and reporter = currentUser()`: "KYB-2,OPS-1",
		`reporter not in (currentUser())`:                    "KYB-1",
		`summary ~ login`:                                    "KYB-1",
		`text ~ "safari"`:                                    "KYB-1",
		`summary !~ "sign"`:                                  "KYB-1,KYB-3,OPS-1",
		`sprint = "Sprint 1"`:                                "KYB-2",
		`sprint in openSprints()`:                            "KYB-2",
		`sprint is empty`:                                    "KYB-1,KYB-3,OPS-1",
		`fixVersion = 1.0`:                                   "KYB-2",
		`fixVersion is not empty`:                            "KYB-2",
		`resolution = "wont_do"`:                             "KYB-3",
		`resolution is empty`:                                "KYB-1,KYB-2,OPS-1",
		`resolution = unresolved`:                            "KYB-1,KYB-2,OPS-1",
		`parent = KYB-2`:                                     "KYB-3",
		`key = kyb-2`:                                        "KYB-2",
		`key in (KYB-1, OPS-1)`:                              "KYB-1,OPS-1",
		`"story points" > 3`:                                 "KYB-2",
		`"story points" >= 3 and "story points" <= 4`:        "KYB-1",
		`"story points" < 5`:                                 "KYB-1",
		`"story points" is empty`:                            "KYB-3,OPS-1",
		`not project = KYB`:                                  "OPS-1",
		`project = KYB and (type = bug or type = story)`:     "KYB-1,KYB-2",
		`project = KYB ORDER BY priority DESC`:               "KYB-1,KYB-2,KYB-3",
		`order by priority`:                                  "KYB-3,KYB-2,KYB-1,OPS-1",
		`ORDER BY key desc`:                                  "OPS-1,KYB-3,KYB-2,KYB-1",
		`order by "story points" desc, key`:                  "KYB-2,KYB-1,KYB-3,OPS-1",
		`order by assignee, status`:                          "KYB-1,OPS-1,KYB-2,KYB-3",
		`order by summary`:                                   "KYB-3,KYB-1,OPS-1,KYB-2",
		`assignee = "nobody@x.uz"`:                           "",
		`assignee = EMPTY`:                                   "KYB-3",
		`assignee in (EMPTY, "bob@x.uz")`:                    "KYB-2,KYB-3",
		`description ~ safari`:                               "KYB-1",
		`description is empty`:                               "KYB-2,KYB-3,OPS-1",
		`summary is not empty and project = OPS`:             "OPS-1",
		`"story points" = 3`:                                 "KYB-1",
		`"story points" != 3`:                                "KYB-2",
		`"story points" = EMPTY`:                             "KYB-3,OPS-1",
		`order by project desc, reporter, type`:              "OPS-1,KYB-2,KYB-3,KYB-1",
		`sprint = "nope"`:                                    "",
	}
	for jql, want := range cases {
		if got := run(t, jql); got != want {
			t.Errorf("%q = %s, want %s", jql, got, want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for jql, pos := range map[string]int{
		`project =`:                 9,
		`project = KYB and`:         17,
		`(project = KYB`:            14,
		`project = KYB)`:            13,
		`colour = red`:              0,
		`project ~ KYB`:             8,
		`"story points" > high`:     17,
		`project in (a, b`:          16,
		`summary in (a)`:            8,
		`project = KYB !`:           14,
		`(project = )`:              11,
		`project KYB`:               8,
		`project in KYB`:            11,
		`project in (,)`:            12,
		`project in (a b)`:          14,
		`assignee = currentUser(x)`: 23,
		`project = "unterminated`:   10,
		`project = KYB order key`:   20,
		`order by colour`:           9,
		`project is red`:            11,
		`assignee = nosuchFunc()`:   11,
		`project = KYB @`:           14,
		`key = 1`:                   6,
	} {
		_, err := domain.Parse(jql)
		var pe *domain.ParseError
		if !errors.As(err, &pe) || !errors.Is(err, domain.ErrInvalidJQL) {
			t.Errorf("%q: err = %v", jql, err)
			continue
		}
		if !strings.HasPrefix(err.Error(), "JQL at ") {
			t.Errorf("%q: message %q", jql, err)
		}
		if pe.Pos != pos {
			t.Errorf("%q: pos = %d (%v), want %d", jql, pe.Pos, err, pos)
		}
	}
	if _, err := domain.Parse(strings.Repeat("(", 2000)); !errors.Is(err, domain.ErrInvalidJQL) {
		t.Fatalf("too long: %v", err)
	}
}
