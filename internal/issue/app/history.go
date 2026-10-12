package app

import (
	"context"
	"encoding/json"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// HistoryLine is a history entry with people's names resolved for display.
type HistoryLine struct {
	domain.HistoryEntry
	ByName           string
	FromName, ToName string // names for the assignee field; the raw values otherwise
}

// OnHistoryEvent projects issue.created / issue.changed / comment.added / issue.imported into the
// issue's history. Redelivery is harmless (entries are keyed by event and position); malformed
// payloads are skipped.
func (s *Service) OnHistoryEvent(ctx context.Context, m outbox.Message) error {
	var e struct {
		ID          string               `json:"id"`
		IssueID     string               `json:"issue_id"`
		Reporter    string               `json:"reporter"`
		Title       string               `json:"title"`
		By          string               `json:"by"`
		Author      string               `json:"author"`
		Body        string               `json:"body"`
		ExternalKey string               `json:"external_key"`
		Items       []domain.FieldChange `json:"items"`
	}
	if json.Unmarshal(m.Payload, &e) != nil {
		return nil
	}
	base := domain.HistoryEntry{Event: m.ID, Issue: domain.IssueID(e.ID), At: m.At}
	var entries []domain.HistoryEntry
	switch m.Name {
	case "issue.created":
		base.By, base.Field, base.To = domain.UserID(e.Reporter), "created", e.Title
		entries = append(entries, base)
	case "issue.imported":
		base.Field, base.To = "imported", e.ExternalKey
		entries = append(entries, base)
	case "comment.added":
		base.Issue, base.By, base.Field, base.To = domain.IssueID(e.IssueID), domain.UserID(e.Author), "comment", e.Body
		entries = append(entries, base)
	case "issue.changed":
		for i, it := range e.Items {
			line := base
			line.Seq, line.By, line.Field, line.From, line.To = i, domain.UserID(e.By), it.Field, it.From, it.To
			entries = append(entries, line)
		}
	}
	if len(entries) == 0 || entries[0].Issue == "" {
		return nil
	}
	return s.history.Record(ctx, entries...)
}

// History returns the issue's full history to anyone who may read the issue.
func (s *Service) History(ctx context.Context, actor, rawKey string) ([]HistoryLine, error) {
	is, err := s.load(ctx, actor, rawKey, false)
	if err != nil {
		return nil, err
	}
	entries, err := s.history.ForIssue(ctx, is.ID())
	if err != nil {
		return nil, err
	}
	names := map[string]string{"": ""}
	name := func(id string) (string, error) {
		if n, ok := names[id]; ok {
			return n, nil
		}
		n, err := s.directory.DisplayName(ctx, id)
		names[id] = n
		return n, err
	}
	out := make([]HistoryLine, 0, len(entries))
	for _, e := range entries {
		l := HistoryLine{HistoryEntry: e, FromName: e.From, ToName: e.To}
		if l.ByName, err = name(string(e.By)); err != nil {
			return nil, err
		}
		if e.Field == "assignee" {
			if l.FromName, err = name(e.From); err != nil {
				return nil, err
			}
			if l.ToName, err = name(e.To); err != nil {
				return nil, err
			}
		}
		out = append(out, l)
	}
	return out, nil
}
