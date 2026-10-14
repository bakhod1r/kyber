// Package httpsend POSTs webhook deliveries. It guards against server-side request forgery: the
// address actually dialed (after DNS, so rebinding cannot sneak past) must not be loopback,
// private, link-local, multicast or unspecified unless private targets are allowed; redirects are
// never followed, no proxy is used, and every request has a short timeout.
package httpsend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/bakhod1r/kyber/internal/webhook/application"
)

// ErrRefused: the target resolves to an address webhooks may not reach.
var ErrRefused = errors.New("webhook target is a private, loopback or link-local address")

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// Blocked reports whether ip is an internal address.
func Blocked(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip) || (ip.To4() != nil && ip.To4()[0] == 0)
}

type Sender struct{ client *http.Client }

// New sends with the given timeout; allowPrivate lifts the address check (tests, trusted LANs).
func New(allowPrivate bool, timeout time.Duration) *Sender {
	dialer := &net.Dialer{Timeout: timeout, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); !allowPrivate && (ip == nil || Blocked(ip)) {
			return ErrRefused
		}
		return nil
	}}
	transport := &http.Transport{Proxy: nil, DialContext: dialer.DialContext, TLSHandshakeTimeout: timeout,
		ResponseHeaderTimeout: timeout, MaxIdleConnsPerHost: 2, IdleConnTimeout: time.Minute}
	return &Sender{client: &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (s *Sender) Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, application.Permanent(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := s.client.Do(req)
	if errors.Is(err, ErrRefused) {
		return 0, application.Permanent(ErrRefused)
	}
	if err != nil {
		return 0, fmt.Errorf("post webhook: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return res.StatusCode, nil
}
