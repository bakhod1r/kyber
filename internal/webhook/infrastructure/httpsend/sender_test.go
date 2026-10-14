package httpsend_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/webhook/application"
	"github.com/bakhod1r/kyber/internal/webhook/infrastructure/httpsend"
)

func TestSend(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		if r.URL.Path == "/slow" {
			time.Sleep(300 * time.Millisecond)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	ctx := context.Background()
	open := httpsend.New(true, 200*time.Millisecond)

	status, err := open.Send(ctx, srv.URL+"/hook", map[string]string{"X-Kyber-Event": "issue.created"}, []byte(`{"a":1}`))
	if err != nil || status != 202 || got.Method != "POST" || got.Header.Get("X-Kyber-Event") != "issue.created" || body != `{"a":1}` {
		t.Fatalf("send = %d %v", status, err)
	}
	if status, err := open.Send(ctx, srv.URL+"/redirect", nil, nil); err != nil || status != 302 || got.URL.Path != "/redirect" {
		t.Fatalf("redirect followed: %d %v %s", status, err, got.URL.Path)
	}
	if _, err := open.Send(ctx, srv.URL+"/slow", nil, nil); err == nil || application.IsPermanent(err) {
		t.Fatalf("timeout: %v", err)
	}
	if _, err := open.Send(ctx, "http://bad host/", nil, nil); err == nil || !application.IsPermanent(err) {
		t.Fatalf("bad url: %v", err)
	}

	guarded := httpsend.New(false, time.Second)
	if _, err := guarded.Send(ctx, srv.URL+"/hook", nil, nil); !errors.Is(err, httpsend.ErrRefused) || !application.IsPermanent(err) {
		t.Fatalf("loopback allowed: %v", err)
	}
	if _, err := guarded.Send(ctx, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)+"/hook", nil, nil); !errors.Is(err, httpsend.ErrRefused) {
		t.Fatalf("localhost allowed: %v", err)
	}
}

func TestBlocked(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "172.16.0.1": true, "192.168.1.1": true, "169.254.169.254": true, "100.64.0.1": true,
		"0.0.0.0": true, "0.1.2.3": true, "224.0.0.1": true, "::1": true, "fe80::1": true, "fc00::1": true, "::": true, "::ffff:10.0.0.1": true,
		"8.8.8.8": false, "2606:4700::1111": false, "100.128.0.1": false,
	} {
		if httpsend.Blocked(net.ParseIP(addr)) != want {
			t.Errorf("Blocked(%s) != %v", addr, want)
		}
	}
}
