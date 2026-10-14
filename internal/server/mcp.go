package server

import (
	"context"
	"encoding/json"
	"errors"

	agileapp "github.com/bakhod1r/kyber/internal/agile/application"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/mcp"
	projectapp "github.com/bakhod1r/kyber/internal/project/application"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	searchapp "github.com/bakhod1r/kyber/internal/search/application"
)

// mcpLimit bounds how many issues one search_issues call returns.
const mcpLimit = 50

// newMCP exposes Kyber to AI assistants (ADR-0009). Every tool calls the same
// application services as the REST API, as the authenticated user.
func newMCP(projects *projectapp.Service, issues *issueapp.Service, search *searchapp.Service, agile *agileapp.Service) *mcp.Server {
	s := mcp.New("kyber", "0.38.0")
	s.Add(mcp.Tool{
		Name: "list_projects", Description: "List the projects you can see, with their keys.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Call: func(ctx context.Context, _ json.RawMessage) (any, error) {
			list, err := projects.List(ctx, projectdomain.UserID(auth.Actor(ctx)))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]string, 0, len(list))
			for _, p := range list {
				out = append(out, map[string]string{"key": p.Key(), "name": p.Name()})
			}
			return map[string]any{"projects": out}, nil
		},
	})
	s.Add(mcp.Tool{
		Name:        "search_issues",
		Description: `Find issues with JQL, e.g. project = KY AND status != done ORDER BY priority DESC. Returns at most 50.`,
		InputSchema: json.RawMessage(`{"type":"object","properties":{"jql":{"type":"string","description":"JQL query"}},"required":["jql"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ JQL string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			res, err := search.Search(ctx, auth.Actor(ctx), in.JQL, mcpLimit)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]string, 0, len(res.Issues))
			for _, is := range res.Issues {
				out = append(out, map[string]string{"key": is.Key, "title": is.Summary, "type": is.Type, "status": is.StatusName,
					"priority": is.Priority, "assignee": is.Assignee})
			}
			return map[string]any{"total": res.Total, "issues": out}, nil
		},
	})
	s.Add(mcp.Tool{
		Name: "get_issue", Description: "Read one issue by key (e.g. KY-12), including its description.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Key string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			is, err := issues.Get(ctx, auth.Actor(ctx), in.Key)
			if err != nil {
				return nil, err
			}
			return issueOut(is), nil
		},
	})
	s.Add(mcp.Tool{
		Name: "create_issue", Description: "Create an issue in a project. type is one of epic, story, task, bug, subtask.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"project":{"type":"string","description":"project key"},` +
			`"title":{"type":"string"},"type":{"type":"string","enum":["epic","story","task","bug","subtask"]},` +
			`"description":{"type":"string"},"parent":{"type":"string","description":"epic or parent issue key"}},"required":["project","title","type"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Project, Title, Type, Description, Parent string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			actor := auth.Actor(ctx)
			is, err := issues.Create(ctx, actor, issueapp.CreateIssue{Project: in.Project, Title: in.Title, Type: in.Type, Parent: in.Parent})
			if err != nil {
				return nil, err
			}
			if in.Description != "" {
				if is, err = issues.Edit(ctx, actor, is.Key().String(), issueapp.EditIssue{Version: is.Version(), Description: &in.Description}); err != nil {
					return nil, err
				}
			}
			return issueOut(is), nil
		},
	})
	s.Add(mcp.Tool{
		Name: "add_comment", Description: "Comment on an issue.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"body":{"type":"string"}},"required":["key","body"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Key, Body string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			c, err := issues.AddComment(ctx, auth.Actor(ctx), in.Key, in.Body)
			if err != nil {
				return nil, err
			}
			return map[string]string{"id": string(c.ID()), "issue": in.Key, "body": c.Body()}, nil
		},
	})
	s.Add(mcp.Tool{
		Name: "get_comments", Description: "Read an issue's comments, oldest first.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Key string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			list, err := issues.Comments(ctx, auth.Actor(ctx), in.Key)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(list))
			for _, c := range list {
				out = append(out, map[string]any{"id": string(c.ID()), "author": c.AuthorName, "body": c.Body(), "created_at": c.CreatedAt()})
			}
			return map[string]any{"comments": out}, nil
		},
	})
	s.Add(mcp.Tool{
		Name:        "transition_issue",
		Description: "Move an issue to another status of its workflow (e.g. in_progress, done). resolution is optional when entering a done status.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"to":{"type":"string","description":"status id"},"resolution":{"type":"string"}},"required":["key","to"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Key, To, Resolution string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			is, err := issues.Transition(ctx, auth.Actor(ctx), in.Key, in.To, in.Resolution)
			if err != nil {
				return nil, err
			}
			return issueOut(is), nil
		},
	})
	s.Add(mcp.Tool{
		Name:        "update_issue",
		Description: "Change an issue's title, description and/or priority (omitted fields stay as they are).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"title":{"type":"string"},"description":{"type":"string"},` +
			`"priority":{"type":"string","enum":["highest","high","medium","low","lowest"]}},"required":["key"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct {
				Key                          string
				Title, Description, Priority *string
			}
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			return edit(ctx, issues, in.Key, func(e *issueapp.EditIssue) {
				e.Title, e.Description, e.Priority = in.Title, in.Description, in.Priority
			})
		},
	})
	s.Add(mcp.Tool{
		Name:        "assign_issue",
		Description: `Assign an issue: assignee is a user id from list_members, "me", or "" to unassign.`,
		InputSchema: json.RawMessage(`{"type":"object","properties":{"key":{"type":"string"},"assignee":{"type":"string"}},"required":["key","assignee"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Key, Assignee string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			if in.Assignee == "me" {
				in.Assignee = auth.Actor(ctx)
			}
			return edit(ctx, issues, in.Key, func(e *issueapp.EditIssue) { e.AssigneeSet, e.Assignee = true, in.Assignee })
		},
	})
	s.Add(mcp.Tool{
		Name: "list_members", Description: "List a project's members (ids, names, roles), e.g. to assign issues.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"project":{"type":"string"}},"required":["project"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Project string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			list, err := projects.Members(ctx, projectdomain.UserID(auth.Actor(ctx)), in.Project)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]string, 0, len(list))
			for _, m := range list {
				out = append(out, map[string]string{"id": string(m.ID), "name": m.Name, "role": string(m.Role)})
			}
			return map[string]any{"members": out}, nil
		},
	})
	s.Add(mcp.Tool{
		Name: "list_sprints", Description: "List a project's sprints with their state (planned, active, completed).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"project":{"type":"string"}},"required":["project"]}`),
		Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ Project string }
			if err := args(raw, &in); err != nil {
				return nil, err
			}
			list, err := agile.List(ctx, auth.Actor(ctx), in.Project)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]string, 0, len(list))
			for _, sp := range list {
				out = append(out, map[string]string{"id": string(sp.ID()), "name": sp.Name(), "goal": sp.Goal(), "state": string(sp.State())})
			}
			return map[string]any{"sprints": out}, nil
		},
	})
	return s
}

// edit applies change to the issue at its current version.
func edit(ctx context.Context, issues *issueapp.Service, key string, change func(*issueapp.EditIssue)) (any, error) {
	actor := auth.Actor(ctx)
	is, err := issues.Get(ctx, actor, key)
	if err != nil {
		return nil, err
	}
	cmd := issueapp.EditIssue{Version: is.Version()}
	change(&cmd)
	if is, err = issues.Edit(ctx, actor, key, cmd); err != nil {
		return nil, err
	}
	return issueOut(is), nil
}

func args(raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return errors.New("arguments do not match the tool's input schema")
	}
	return nil
}

func issueOut(is *issuedomain.Issue) map[string]any {
	return map[string]any{"key": is.Key().String(), "title": is.Title(), "type": string(is.Type()), "status": string(is.Status()),
		"priority": string(is.Priority()), "assignee": string(is.Assignee()), "description": is.Description()}
}
