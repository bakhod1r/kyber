// Package livekit is the media-server adapter for LiveKit (ADR-0017): it signs short-lived access tokens
// (HS256 JWTs with LiveKit's video grant) that let one user join one call's room. Kyber decides who may join;
// LiveKit only checks the signature. The API secret never leaves the server and is never logged.
package livekit

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/call/application"
)

type Config struct {
	URL    string        // where browsers connect, ws:// or wss:// (KYBER_LIVEKIT_URL)
	APIURL string        // where Kyber calls LiveKit's server API (KYBER_LIVEKIT_API_URL); default URL over http(s)
	Key    string        // API key (KYBER_LIVEKIT_KEY)
	Secret string        // API secret (KYBER_LIVEKIT_SECRET)
	TTL    time.Duration // token lifetime; LiveKit refreshes it for connected clients (default 15 minutes)
	// Recordings (S104): LiveKit Egress writes MP4 files into EgressDir (its own path, default /out); Kyber reads
	// the same directory at RecordingsDir (KYBER_LIVEKIT_RECORDINGS_DIR). Unset, rooms are recorded in the browser.
	RecordingsDir string
	EgressDir     string
	Wait, Poll    time.Duration // how long Open waits for a recording to finish, and how often it asks (30 s, 1 s)
}

var ErrConfig = errors.New("livekit: set KYBER_LIVEKIT_URL (ws:// or wss://), KYBER_LIVEKIT_KEY and KYBER_LIVEKIT_SECRET")

type LiveKit struct {
	c    Config
	url  *url.URL
	http *http.Client
}

func New(c Config) (*LiveKit, error) {
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || c.Key == "" || c.Secret == "" {
		return nil, ErrConfig
	}
	if c.APIURL != "" {
		if a, err := url.Parse(c.APIURL); err != nil || (a.Scheme != "http" && a.Scheme != "https") || a.Host == "" {
			return nil, ErrConfig
		}
	}
	c.TTL = cmp.Or(c.TTL, 15*time.Minute)
	c.EgressDir = cmp.Or(c.EgressDir, "/out")
	c.Wait = cmp.Or(c.Wait, 30*time.Second)
	c.Poll = cmp.Or(c.Poll, time.Second)
	return &LiveKit{c: c, url: u, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

// Origins are the addresses the web app talks to (its WebSocket and LiveKit's HTTP fallback), for the CSP.
func (l *LiveKit) Origins() []string {
	web := "https"
	if l.url.Scheme == "ws" {
		web = "http"
	}
	return []string{l.url.Scheme + "://" + l.url.Host, web + "://" + l.url.Host}
}

type claims struct {
	Iss   string         `json:"iss"`
	Sub   string         `json:"sub"`
	Nbf   int64          `json:"nbf"`
	Exp   int64          `json:"exp"`
	Video map[string]any `json:"video"`
}

func claimsWith(key, sub string, now time.Time, ttl time.Duration, grant map[string]any) claims {
	return claims{Iss: key, Sub: sub, Nbf: now.Unix() - 10, Exp: now.Add(ttl).Unix(), Video: grant}
}

// sign makes the HS256 JWT LiveKit expects.
func (l *LiveKit) sign(c claims) string {
	body, _ := json.Marshal(c) // strings, numbers and bools always marshal
	signing := header + "." + base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(l.c.Secret))
	mac.Write([]byte(signing))
	return strings.Join([]string{signing, base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}, ".")
}

var header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Ticket lets user join room (a call id), publish and subscribe; nothing more (no room admin).
func (l *LiveKit) Ticket(room, user string, now time.Time) (application.RoomTicket, error) {
	exp := now.Add(l.c.TTL)
	token := l.sign(claimsWith(l.c.Key, user, now, l.c.TTL, map[string]any{
		"room": room, "roomJoin": true, "canPublish": true, "canSubscribe": true, "canPublishData": true}))
	return application.RoomTicket{URL: l.c.URL, Token: token, Expires: exp}, nil
}
