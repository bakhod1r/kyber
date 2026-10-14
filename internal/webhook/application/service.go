// Package application holds the Webhook context's use cases: project admins manage outgoing
// webhooks, project events are queued as signed deliveries and sent with retries, and commits and
// pull requests from Git hosting become development links on the issues they mention.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrIssueNotFound   = errors.New("issue not found")
	ErrForbidden       = errors.New("insufficient project role")
)

// Projects checks that the actor administers a project (ACL to Project/Access). Hidden and
// missing projects give ErrProjectNotFound, other roles ErrForbidden.
type Projects interface {
	AuthorizeAdmin(ctx context.Context, actor, project string) error
}

// Issues checks that the actor may browse an issue and returns its canonical key (ACL to Issue
// Tracking). Hidden and missing issues give ErrIssueNotFound.
type Issues interface {
	Authorize(ctx context.Context, actor, issue string) (string, error)
}

// Sender POSTs body to url with the given headers and returns the response status. Return
// Permanent(err) when retrying cannot help (a refused target).
type Sender interface {
	Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error)
}

type permanentError struct{ error }

func (p permanentError) Unwrap() error { return p.error }

// Permanent marks a send error as final: the delivery is not retried.
func Permanent(err error) error { return permanentError{err} }

// IsPermanent tells whether err was marked with Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

type Deps struct {
	Webhooks   domain.Repository
	Deliveries domain.DeliveryRepository
	Links      domain.LinkRepository
	Projects   Projects
	Issues     Issues
	Sender     Sender
	NewID      func() string
	NewSecret  func() string
	Now        func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Create adds an active webhook to the project. The returned secret is shown only this once.
func (s *Service) Create(ctx context.Context, actor, project, url string, events []string) (*domain.Webhook, error) {
	if err := s.d.Projects.AuthorizeAdmin(ctx, actor, project); err != nil {
		return nil, err
	}
	w, err := domain.New(domain.ID(s.d.NewID()), project, url, s.d.NewSecret(), events, actor, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	return w, s.d.Webhooks.Save(ctx, w)
}

// Webhooks lists the project's webhooks, oldest first.
func (s *Service) Webhooks(ctx context.Context, actor, project string) ([]*domain.Webhook, error) {
	if err := s.d.Projects.AuthorizeAdmin(ctx, actor, project); err != nil {
		return nil, err
	}
	return s.d.Webhooks.ForProject(ctx, project)
}

// load finds a webhook of the project the actor administers.
func (s *Service) load(ctx context.Context, actor, project, id string) (*domain.Webhook, error) {
	if err := s.d.Projects.AuthorizeAdmin(ctx, actor, project); err != nil {
		return nil, err
	}
	w, err := s.d.Webhooks.ByID(ctx, domain.ID(id))
	if err != nil {
		return nil, err
	}
	if w.Project != project {
		return nil, domain.ErrWebhookNotFound
	}
	return w, nil
}

// Update changes the webhook's URL, events and active flag (the secret stays).
func (s *Service) Update(ctx context.Context, actor, project, id, url string, events []string, active bool) (*domain.Webhook, error) {
	w, err := s.load(ctx, actor, project, id)
	if err != nil {
		return nil, err
	}
	if err := w.Reconfigure(url, events, active); err != nil {
		return nil, err
	}
	return w, s.d.Webhooks.Save(ctx, w)
}

// Delete removes the webhook and its delivery history.
func (s *Service) Delete(ctx context.Context, actor, project, id string) error {
	w, err := s.load(ctx, actor, project, id)
	if err != nil {
		return err
	}
	return s.d.Webhooks.Delete(ctx, w.ID)
}

// RecentDeliveries is how many deliveries Deliveries shows.
const RecentDeliveries = 20

// Deliveries lists the webhook's latest deliveries, newest first.
func (s *Service) Deliveries(ctx context.Context, actor, project, id string) ([]domain.Delivery, error) {
	w, err := s.load(ctx, actor, project, id)
	if err != nil {
		return nil, err
	}
	return s.d.Deliveries.Recent(ctx, w.ID, RecentDeliveries)
}

// safeFields are the only event payload fields a delivery carries: identifiers, never titles,
// descriptions, comment bodies or user data. Event payloads are written for in-process handlers
// and include content of issues that security levels may hide from some project members; a
// webhook receiver sees only that something happened to which issue and fetches details through
// the authorized API (ADR-0014).
var safeFields = []string{"id", "key", "issue_id", "issue_key", "project", "sprint_id", "version_id", "meeting_id", "board_id"}

// Payload is the JSON document POSTed to a webhook.
type Payload struct {
	ID         string         `json:"id"`
	Event      string         `json:"event"`
	Project    string         `json:"project"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       map[string]any `json:"data"`
}

// eventData reads the project of an event and the identifiers it may share.
func eventData(payload []byte) (string, map[string]any) {
	var all map[string]any
	if json.Unmarshal(payload, &all) != nil {
		return "", nil
	}
	data := map[string]any{}
	for _, f := range safeFields {
		switch v := all[f].(type) {
		case string, float64:
			data[f] = v
		}
	}
	project, _ := data["project"].(string)
	for _, f := range []string{"key", "issue_key"} {
		if k, ok := data[f].(string); ok && project == "" {
			if i := strings.LastIndexByte(k, '-'); i > 0 {
				project = k[:i]
			}
		}
	}
	if project != "" {
		data["project"] = project
	}
	return project, data
}

// OnEvent is the outbox handler (for every event): it queues one delivery per matching active
// webhook of the event's project. Keys derive from the outbox message, so redelivery is harmless.
func (s *Service) OnEvent(ctx context.Context, m outbox.Message) error {
	project, data := eventData(m.Payload)
	if project == "" {
		return nil
	}
	hooks, err := s.d.Webhooks.ForProject(ctx, project)
	if err != nil {
		return err
	}
	for _, w := range hooks {
		if !w.Wants(m.Name) {
			continue
		}
		id := s.d.NewID()
		body, _ := json.Marshal(Payload{ID: id, Event: m.Name, Project: project, OccurredAt: m.At.UTC(), Data: data})
		d := domain.NewDelivery(id, tenant.Workspace(ctx), w.ID, m.Name, fmt.Sprintf("%d/%s", m.ID, w.ID), body, s.d.Now().UTC())
		if err := s.d.Deliveries.Enqueue(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

const (
	batch = 20
	lease = 2 * time.Minute // longer than the sender's timeout
)

// Dispatch sends one batch of due deliveries and returns how many it attempted.
func (s *Service) Dispatch(ctx context.Context) (int, error) {
	due, err := s.d.Deliveries.Claim(ctx, s.d.Now().UTC(), lease, batch)
	if err != nil {
		return 0, err
	}
	for _, d := range due {
		wctx := tenant.With(ctx, d.Workspace)
		w, err := s.d.Webhooks.ByID(wctx, d.Webhook)
		switch {
		case errors.Is(err, domain.ErrWebhookNotFound):
			continue // deleted meanwhile, with its deliveries
		case err != nil:
			return 0, err
		case !w.Active:
			d.Failed(0, "webhook is inactive", true, s.d.Now().UTC())
		default:
			s.send(ctx, w, &d)
		}
		if err := s.d.Deliveries.Save(wctx, d); err != nil {
			return 0, err
		}
	}
	return len(due), nil
}

func (s *Service) send(ctx context.Context, w *domain.Webhook, d *domain.Delivery) {
	status, err := s.d.Sender.Send(ctx, w.URL, map[string]string{
		"Content-Type":      "application/json",
		"User-Agent":        "Kyber-Webhook/1",
		"X-Kyber-Event":     d.Event,
		"X-Kyber-Delivery":  d.ID,
		"X-Kyber-Signature": domain.Sign(w.Secret, d.Body),
	}, d.Body)
	now := s.d.Now().UTC()
	switch {
	case err != nil:
		d.Failed(status, err.Error(), IsPermanent(err), now)
	case status < 200 || status > 299:
		d.Failed(status, "HTTP "+strconv.Itoa(status), false, now)
	default:
		d.Delivered(status, now)
	}
}

// Development lists the commits and pull requests that mention the issue.
func (s *Service) Development(ctx context.Context, actor, issue string) ([]domain.Link, error) {
	key, err := s.d.Issues.Authorize(ctx, actor, issue)
	if err != nil {
		return nil, err
	}
	return s.d.Links.ForIssue(ctx, key)
}
