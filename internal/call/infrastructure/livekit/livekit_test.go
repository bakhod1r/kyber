package livekit_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/call/infrastructure/livekit"
)

func TestTicket(t *testing.T) {
	lk, err := livekit.New(livekit.Config{URL: "wss://sfu.example.com", Key: "APIkey", Secret: "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	tk, err := lk.Ticket("call-1", "user-1", now)
	if err != nil || tk.URL != "wss://sfu.example.com" || !tk.Expires.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("ticket = %+v %v", tk, err)
	}
	parts := strings.Split(tk.Token, ".")
	if len(parts) != 3 {
		t.Fatalf("token = %q", tk.Token)
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		t.Fatal("bad signature")
	}
	var head struct{ Alg, Typ string }
	raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if err := json.Unmarshal(raw, &head); err != nil || head.Alg != "HS256" || head.Typ != "JWT" {
		t.Fatalf("header = %s", raw)
	}
	var claims struct {
		Iss, Sub string
		Nbf, Exp int64
		Video    struct {
			Room                                               string
			RoomJoin, CanPublish, CanSubscribe, CanPublishData bool
			RoomAdmin                                          bool
		}
	}
	raw, _ = base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	v := claims.Video
	if claims.Iss != "APIkey" || claims.Sub != "user-1" || claims.Nbf != now.Unix()-10 || claims.Exp != now.Add(15*time.Minute).Unix() ||
		v.Room != "call-1" || !v.RoomJoin || !v.CanPublish || !v.CanSubscribe || !v.CanPublishData || v.RoomAdmin {
		t.Fatalf("claims = %s", raw)
	}
	if strings.Contains(string(raw), "s3cret") {
		t.Fatal("the secret leaked into the token")
	}
}

func TestConfig(t *testing.T) {
	for name, c := range map[string]livekit.Config{
		"no url":     {Key: "k", Secret: "s"},
		"http url":   {URL: "https://sfu.example.com", Key: "k", Secret: "s"},
		"bad url":    {URL: "wss://", Key: "k", Secret: "s"},
		"no key":     {URL: "wss://sfu.example.com", Secret: "s"},
		"no secret":  {URL: "wss://sfu.example.com", Key: "k"},
		"weird char": {URL: "wss://sfu.example.com/\n", Key: "k", Secret: "s"},
	} {
		if _, err := livekit.New(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	lk, err := livekit.New(livekit.Config{URL: "ws://localhost:7880", Key: "k", Secret: "s", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if tk, _ := lk.Ticket("r", "u", time.Unix(0, 0)); !tk.Expires.Equal(time.Unix(60, 0)) {
		t.Fatalf("ttl = %v", tk.Expires)
	}
	if got := lk.Origins(); len(got) != 2 || got[0] != "ws://localhost:7880" || got[1] != "http://localhost:7880" {
		t.Fatalf("origins = %v", got)
	}
}
