package server

import (
	"context"
	"encoding/json"
	"errors"

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
func newMCP(projects *projectapp.Service, issues *issueapp.Service, search *searchapp.Service) *mcp.Server {
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
	return s
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
