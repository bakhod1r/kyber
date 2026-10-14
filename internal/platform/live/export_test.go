package live

import "time"

// SetRetry shortens the listener's reconnect pause in tests.
func SetRetry(p *Postgres, d time.Duration) { p.retry = d }
