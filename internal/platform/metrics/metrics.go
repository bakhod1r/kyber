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
	m.mu.Unlock()
}
