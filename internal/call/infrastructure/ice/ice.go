// Package ice hands browsers the STUN/TURN servers for WebRTC (ADR-0007). TURN credentials are
// short-lived and follow coturn's REST scheme (use-auth-secret): username "expiry:user",
// credential base64(HMAC-SHA1(secret, username)).
package ice

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	URLs   []string      // stun:, turn: and turns: URLs (KYBER_ICE_SERVERS)
	Secret string        // coturn static-auth-secret (KYBER_TURN_SECRET); TURN is offered only with it
	TTL    time.Duration // credential lifetime
}

// Server is one RTCIceServer as the browser expects it.
type Server struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// Parse splits a comma-separated list of URLs.
func Parse(list string) []string {
	var out []string
	for _, u := range strings.Split(list, ",") {
		if u = strings.TrimSpace(u); u != "" {
			out = append(out, u)
		}
	}
	return out
}

func (c Config) Servers(user string, now time.Time) []Server {
	out := []Server{}
	var stun, turn []string
	for _, u := range c.URLs {
		if strings.HasPrefix(u, "turn:") || strings.HasPrefix(u, "turns:") {
			turn = append(turn, u)
		} else {
			stun = append(stun, u)
		}
	}
	if len(stun) > 0 {
		out = append(out, Server{URLs: stun})
	}
	if len(turn) > 0 && c.Secret != "" {
		ttl := c.TTL
		if ttl == 0 {
			ttl = 12 * time.Hour
		}
		name := strconv.FormatInt(now.Add(ttl).Unix(), 10) + ":" + user
		mac := hmac.New(sha1.New, []byte(c.Secret))
		mac.Write([]byte(name))
		out = append(out, Server{URLs: turn, Username: name, Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil))})
	}
	return out
}
