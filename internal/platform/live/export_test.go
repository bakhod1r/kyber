package live

import "time"

// Subscribe exposes a raw hub subscription to the external tests.
func Subscribe(h *Hub, workspace string) (<-chan Change, func()) {
	s, cancel := h.subscribe(workspace)
	return s.ch, cancel
}

// SetRetry shortens the listener's reconnect pause in tests.
func SetRetry(p *Postgres, d time.Duration) { p.retry = d }
