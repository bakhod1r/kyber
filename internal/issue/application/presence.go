package application

import (
	"context"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// PresenceTTL is how long a heartbeat keeps someone listed as viewing an issue (clients beat every 20 s).
const PresenceTTL = 45 * time.Second

// PresenceHints tells subscribers of a project that who-is-viewing changed (a live hint, best effort).
type PresenceHints interface {
	PresenceChanged(ctx context.Context, project string)
}

// Viewer is someone looking at an issue right now.
type Viewer struct {
	ID, Name string
}

// Presence tracks who is viewing which issue. The registry is per replica: behind a load balancer
// each replica only knows the viewers whose heartbeats it received.
type Presence struct {
	issues   *Service
	registry domain.PresenceRegistry
	hints    PresenceHints
	now      func() time.Time
}

func NewPresence(issues *Service, registry domain.PresenceRegistry, hints PresenceHints, now func() time.Time) *Presence {
	return &Presence{issues: issues, registry: registry, hints: hints, now: now}
}

// Heartbeat records that the actor views the issue (anyone who can see it).
func (p *Presence) Heartbeat(ctx context.Context, actor, rawKey string) error {
	is, err := p.issues.load(ctx, actor, rawKey, PermBrowse)
	if err != nil {
		return err
	}
	now := p.now()
	arrived, err := p.registry.Seen(ctx, is.ID(), domain.UserID(actor), now, now.Add(PresenceTTL))
	if err != nil {
		return err
	}
	if arrived {
		p.hints.PresenceChanged(ctx, is.Key().Project())
	}
	return nil
}

// Viewers lists who is viewing the issue now, by name, in order of arrival.
func (p *Presence) Viewers(ctx context.Context, actor, rawKey string) ([]Viewer, error) {
	is, err := p.issues.load(ctx, actor, rawKey, PermBrowse)
	if err != nil {
		return nil, err
	}
	ids, err := p.registry.Present(ctx, is.ID(), p.now())
	if err != nil {
		return nil, err
	}
	out := make([]Viewer, 0, len(ids))
	for _, id := range ids {
		name, err := p.issues.directory.DisplayName(ctx, string(id))
		if err != nil {
			return nil, err
		}
		out = append(out, Viewer{ID: string(id), Name: name})
	}
	return out, nil
}
