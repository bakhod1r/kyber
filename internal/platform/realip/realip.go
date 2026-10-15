// Package realip recovers the client address of requests that reach Kyber through trusted reverse proxies.
package realip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Parse reads a comma-separated list of CIDRs or single addresses (KYBER_TRUSTED_PROXIES).
func Parse(csv string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.Split(csv, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.Contains(f, "/") {
			a, err := netip.ParseAddr(f)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", f, err)
			}
			out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(f)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", f, err)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// Middleware rewrites r.RemoteAddr to the client address when the TCP peer is a trusted proxy. It walks
// X-Forwarded-For from the right and takes the first address that is not itself a trusted proxy, so a
// client cannot choose its address by sending its own header. An unparseable entry stops the walk and the
// peer stays the address. With no trusted proxies the header is ignored.
func Middleware(trusted []netip.Prefix, next http.Handler) http.Handler {
	if len(trusted) == 0 {
		return next
	}
	isTrusted := func(a netip.Addr) bool {
		a = a.Unmap()
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client, ok := forwardedClient(r, isTrusted); ok {
			r2 := r.Clone(r.Context())
			r2.RemoteAddr = net.JoinHostPort(client.String(), "0")
			r = r2
		}
		next.ServeHTTP(w, r)
	})
}

func forwardedClient(r *http.Request, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !isTrusted(peer.Addr()) {
		return netip.Addr{}, false
	}
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	var last netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		a = a.Unmap()
		if !isTrusted(a) {
			return a, true
		}
		last = a
	}
	return last, last.IsValid()
}
