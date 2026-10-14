// Package livekit is the media-server adapter for LiveKit (ADR-0017): it signs short-lived access tokens
// (HS256 JWTs with LiveKit's video grant) that let one user join one call's room. Kyber decides who may join;
// LiveKit only checks the signature. The API secret never leaves the server and is never logged.
package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/call/application"
)

type Config struct {
	URL    string        // where browsers connect, ws:// or wss:// (KYBER_LIVEKIT_URL)
	Key    string        // API key (KYBER_LIVEKIT_KEY)
	Secret string        // API secret (KYBER_LIVEKIT_SECRET)
	TTL    time.Duration // token lifetime; LiveKit refreshes it for connected clients (default 15 minutes)
}

var ErrConfig = errors.New("livekit: set KYBER_LIVEKIT_URL (ws:// or wss://), KYBER_LIVEKIT_KEY and KYBER_LIVEKIT_SECRET")

type LiveKit struct {
	c   Config
	url *url.URL
}

func New(c Config) (*LiveKit, error) {
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || c.Key == "" || c.Secret == "" {
		return nil, ErrConfig
	}
	if c.TTL == 0 {
		c.TTL = 15 * time.Minute
	}
	return &LiveKit{c: c, url: u}, nil
}

// Origins are the addresses the web app talks to (its WebSocket and LiveKit's HTTP fallback), for the CSP.
func (l *LiveKit) Origins() []string {
	web := "https"
	if l.url.Scheme == "ws" {
		web = "http"
	}
	return []string{l.url.Scheme + "://" + l.url.Host, web + "://" + l.url.Host}
}

type grant struct {
	Room           string `json:"room"`
	RoomJoin       bool   `json:"roomJoin"`
	CanPublish     bool   `json:"canPublish"`
	CanSubscribe   bool   `json:"canSubscribe"`
	CanPublishData bool   `json:"canPublishData"`
}

type claims struct {
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Nbf   int64  `json:"nbf"`
	Exp   int64  `json:"exp"`
	Video grant  `json:"video"`
}

var header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Ticket lets user join room (a call id), publish and subscribe; nothing more (no room admin).
func (l *LiveKit) Ticket(room, user string, now time.Time) (application.RoomTicket, error) {
	exp := now.Add(l.c.TTL)
	body, _ := json.Marshal(claims{ // strings, numbers and bools always marshal
		Iss: l.c.Key, Sub: user, Nbf: now.Unix() - 10, Exp: exp.Unix(),
		Video: grant{Room: room, RoomJoin: true, CanPublish: true, CanSubscribe: true, CanPublishData: true}})
	signing := header + "." + base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(l.c.Secret))
	mac.Write([]byte(signing))
	token := strings.Join([]string{signing, base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}, ".")
	return application.RoomTicket{URL: l.c.URL, Token: token, Expires: exp}, nil
}
