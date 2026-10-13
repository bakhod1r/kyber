package smtpmail_test

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/smtpmail"
)

// fakeSMTP speaks just enough SMTP for net/smtp and records the DATA of each message.
type fakeSMTP struct {
	ln       net.Listener
	mu       sync.Mutex
	messages []string
	rcpt     []string
	reject   string // reply to RCPT TO, e.g. "550 no such user"
	auth     []string
}

func startFake(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			say("250-fake")
			say("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH"):
			f.mu.Lock()
			f.auth = append(f.auth, strings.TrimSpace(line))
			f.mu.Unlock()
			say("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			if f.reject != "" {
				say(f.reject)
				continue
			}
			f.mu.Lock()
			f.rcpt = append(f.rcpt, strings.TrimSpace(line))
			f.mu.Unlock()
			say("250 ok")
		case cmd == "DATA":
			say("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.messages = append(f.messages, b.String())
			f.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

func cfg(f *fakeSMTP) smtpmail.Config {
	host, port, _ := net.SplitHostPort(f.ln.Addr().String())
	return smtpmail.Config{Host: host, Port: port, Username: "kyber", Password: "secret", From: "Kyber <noreply@kyber.example>", TLS: smtpmail.TLSNone}
}

func delivery(t *testing.T, to, subject string) domain.Delivery {
	t.Helper()
	d, err := domain.NewDelivery("d-1", "ws", domain.ChannelEmail, to, subject, "Aziza assigned you KYB-1 «Login».\n\nhttps://kyber.example/x", "k", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSendsAPlainTextMessage(t *testing.T) {
	f := startFake(t)
	s, err := smtpmail.New(cfg(f))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), delivery(t, "bob@x.uz", "[KYB-1] Aziza assigned you: «Login»")); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) != 1 || len(f.rcpt) != 1 || !strings.Contains(f.rcpt[0], "<bob@x.uz>") || len(f.auth) != 1 {
		t.Fatalf("rcpt=%v auth=%v messages=%d", f.rcpt, f.auth, len(f.messages))
	}
	m := f.messages[0]
	for _, want := range []string{
		"From: \"Kyber\" <noreply@kyber.example>\r\n", "To: <bob@x.uz>\r\n", "Subject: =?utf-8?q?", "MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n", "Message-Id: <", "Date: ", "Auto-Submitted: auto-generated\r\n",
		"KYB-1 =C2=ABLogin=C2=BB.",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("message lacks %q:\n%s", want, m)
		}
	}
}

func TestHeaderInjectionIsRefused(t *testing.T) {
	f := startFake(t)
	s, _ := smtpmail.New(cfg(f))
	err := s.Send(context.Background(), delivery(t, "bob@x.uz\r\nBcc: eve@x.uz", "s"))
	if !application.IsPermanent(err) {
		t.Fatalf("injection err = %v (must be permanent)", err)
	}
	if err := s.Send(context.Background(), delivery(t, "bob@x.uz", "hi\r\nBcc: eve@x.uz")); err != nil {
		t.Fatalf("subject newlines are folded away, not an error: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.Contains(f.messages[0], "\r\nBcc:") {
		t.Fatalf("header injected:\n%s", f.messages[0])
	}
}

func TestPermanentAndTemporaryFailures(t *testing.T) {
	f := startFake(t)
	s, _ := smtpmail.New(cfg(f))
	f.reject = "550 5.1.1 no such user"
	err := s.Send(context.Background(), delivery(t, "nobody@x.uz", "s"))
	if !isPermanent(err) {
		t.Fatalf("550 must be permanent: %v", err)
	}
	f.reject = "451 4.3.0 try again later"
	if err := s.Send(context.Background(), delivery(t, "bob@x.uz", "s")); err == nil || isPermanent(err) {
		t.Fatalf("451 must be temporary: %v", err)
	}
	cl := cfg(f)
	cl.Port = "1" // nothing listens: network errors are temporary
	down, _ := smtpmail.New(cl)
	if err := down.Send(context.Background(), delivery(t, "bob@x.uz", "s")); err == nil || isPermanent(err) {
		t.Fatalf("connection refused must be temporary: %v", err)
	}
}

func TestConfig(t *testing.T) {
	if _, err := smtpmail.New(smtpmail.Config{Host: "smtp.x", From: "not an address"}); err == nil {
		t.Fatal("bad From must be refused")
	}
	if _, err := smtpmail.New(smtpmail.Config{Host: "smtp.x", From: "a@x.uz", TLS: "maybe"}); err == nil {
		t.Fatal("bad TLS mode must be refused")
	}
	c, err := smtpmail.New(smtpmail.Config{Host: "smtp.x", From: "a@x.uz"})
	if err != nil || c == nil {
		t.Fatalf("defaults: %v", err)
	}
}

func isPermanent(err error) bool { return application.IsPermanent(err) }
