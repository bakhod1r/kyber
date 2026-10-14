// Package metrics exposes minimal HTTP metrics in Prometheus text format.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

type key struct {
	method string
	status int
}

type HTTP struct {
	mu       sync.Mutex
	count    map[key]uint64
	duration map[key]float64
	gauges   []gauge
}

type gauge struct {
	name, help string
	read       func() float64
}

// Gauge exposes a value read at scrape time (e.g. open streams, live calls).
func (m *HTTP) Gauge(name, help string, read func() float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges = append(m.gauges, gauge{name, help, read})
}

func NewHTTP() *HTTP { return &HTTP{count: map[key]uint64{}, duration: map[key]float64{}} }

func (m *HTTP) Observe(method string, status int, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key{method, status}
	m.count[k]++
	m.duration[k] += d.Seconds()
}

func (m *HTTP) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	keys := make([]key, 0, len(m.count))
	for k := range m.count {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].status < keys[j].status
	})
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# TYPE kyber_http_requests_total counter")
	for _, k := range keys {
		fmt.Fprintf(w, "kyber_http_requests_total{method=%q,status=\"%d\"} %d\n", k.method, k.status, m.count[k])
	}
	fmt.Fprintln(w, "# TYPE kyber_http_request_duration_seconds_sum counter")
	for _, k := range keys {
		fmt.Fprintf(w, "kyber_http_request_duration_seconds_sum{method=%q,status=\"%d\"} %s\n",
			k.method, k.status, strconv.FormatFloat(m.duration[k], 'f', -1, 64))
	}
	gauges := m.gauges
	m.mu.Unlock()
	for _, g := range gauges {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", g.name, g.help, g.name, g.name, strconv.FormatFloat(g.read(), 'f', -1, 64))
	}
}
