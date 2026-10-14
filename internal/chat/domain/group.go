package domain

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

// GroupID identifies a user group.
type GroupID string

// Group is a named set of people in a workspace (S70): mentioning @handle notifies its members.
type Group struct {
	ID        GroupID
	Workspace WorkspaceID
	Handle    string
	Name      string
	Members   []UserID // sorted, unique
	CreatedBy UserID
	CreatedAt time.Time
}

var (
	ErrInvalidHandle = errors.New("a group handle is 2–30 lower-case letters, digits or dashes and not a reserved word")
	ErrHandleTaken   = errors.New("that group handle is taken")
	ErrGroupNotFound = errors.New("group not found")
)

var (
	handleRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,29}$`)
	groupMentionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9._%+-])@([a-z0-9][a-z0-9-]{1,29})\b([^@.A-Za-z0-9]|$|\.(?:\s|$))`)
	reserved       = []string{"here", "channel", "everyone", "all"}
)

func members(us []UserID) []UserID {
	out := slices.Clone(us)
	slices.Sort(out)
	return slices.Compact(out)
}

func handle(h string) (string, error) {
	h = strings.ToLower(strings.TrimSpace(h))
	if !handleRe.MatchString(h) || slices.Contains(reserved, h) {
		return "", ErrInvalidHandle
	}
	return h, nil
}

func NewGroup(id GroupID, ws WorkspaceID, h, name string, people []UserID, by UserID, now time.Time) (*Group, error) {
	h, err := handle(h)
	if err != nil {
		return nil, err
	}
	return &Group{ID: id, Workspace: ws, Handle: h, Name: strings.TrimSpace(name), Members: members(people), CreatedBy: by, CreatedAt: now.UTC()}, nil
}

// Change renames the group and replaces its members.
func (g *Group) Change(h, name string, people []UserID) error {
	h, err := handle(h)
	if err != nil {
		return err
	}
	g.Handle, g.Name, g.Members = h, strings.TrimSpace(name), members(people)
	return nil
}

// GroupMentions lists the distinct @handles in a message body, in order (e-mail mentions are not handles).
func GroupMentions(body string) []string {
	var out []string
	for _, m := range groupMentionRe.FindAllStringSubmatch(body, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// Groups stores user groups.
type Groups interface {
	Save(ctx context.Context, g *Group) error // insert or update; ErrHandleTaken
	Delete(ctx context.Context, id GroupID) error
	ByID(ctx context.Context, id GroupID) (*Group, error) // ErrGroupNotFound
	ByHandle(ctx context.Context, ws WorkspaceID, handle string) (*Group, error)
	// In lists the workspace's groups by handle.
	In(ctx context.Context, ws WorkspaceID) ([]*Group, error)
}
