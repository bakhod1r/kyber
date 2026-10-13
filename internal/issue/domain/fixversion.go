package domain

import (
	"slices"
	"strings"
)

// VersionID is a project version (release) of the Agile context, referenced by id.
type VersionID string

// FixVersions are the releases the issue is planned to ship in, sorted.
func (i *Issue) FixVersions() []VersionID { return slices.Clone(i.fixVersions) }

// HasFixVersion reports whether the issue is planned into the version.
func (i *Issue) HasFixVersion(v VersionID) bool { return slices.Contains(i.fixVersions, v) }

// PlanForVersions replaces the issue's fix versions (duplicates are dropped).
func (i *Issue) PlanForVersions(ids []VersionID) {
	out := slices.Clone(ids)
	slices.Sort(out)
	i.fixVersions = slices.Compact(out)
}

// MoveToVersion moves the issue from one version to another ("" = just remove it).
func (i *Issue) MoveToVersion(from, to VersionID) {
	ids := slices.DeleteFunc(i.FixVersions(), func(v VersionID) bool { return v == from })
	if to != "" {
		ids = append(ids, to)
	}
	i.PlanForVersions(ids)
}

func joinVersions(ids []VersionID) string {
	s := make([]string, len(ids))
	for n, id := range ids {
		s[n] = string(id)
	}
	return strings.Join(s, ",")
}
