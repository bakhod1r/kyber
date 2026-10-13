// Package smtpmail sends notification emails through any SMTP server (Gmail, Mailgun, Postfix…).
package smtpmail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// TLS modes: STARTTLS on a plain port (587, default), implicit TLS (465), or none (local relays only).
const (
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"
	TLSNone     = "none"
)

type Config struct {
	Host, Port         string
	Username, Password string
	From               string // e.g. "Kyber <noreply@kyber.example>"
	TLS                string
}

type Sender struct {
	cfg  Config
	from *mail.Address
}

func New(c Config) (*Sender, error) {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return nil, fmt.Errorf("KYBER_SMTP_FROM: %w", err)
	}
	if c.TLS == "" {
		c.TLS = TLSStartTLS
	}
	if c.TLS != TLSStartTLS && c.TLS != TLSImplicit && c.TLS != TLSNone {
		return nil, fmt.Errorf("KYBER_SMTP_TLS must be starttls, tls or none, not %q", c.TLS)
	}
	if c.Port == "" {
		c.Port = map[string]string{TLSImplicit: "465", TLSStartTLS: "587", TLSNone: "25"}[c.TLS]
	}
	return &Sender{cfg: c, from: from}, nil
}

const timeout = 30 * time.Second

func (s *Sender) Send(ctx context.Context, d domain.Delivery) error {
	to, err := mail.ParseAddress(d.To)
	if err != nil || strings.ContainsAny(d.To, "\r\n") {
		return application.Permanent(fmt.Errorf("invalid recipient address: %q", d.To))
	}
	msg := s.message(to, d)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = s.send(ctx, to.Address, msg)
	var tp *textproto.Error
	if errors.As(err, &tp) && tp.Code >= 500 {
		return application.Permanent(err) // 5xx: the server refuses for good (unknown user, rejected sender…)
	}
	return err
}

func (s *Sender) send(ctx context.Context, to string, msg []byte) error {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	if s.cfg.TLS == TLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.cfg.TLS == TLSStartTLS {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.cfg.Username != "" {
		// PlainAuth refuses to send credentials over an unencrypted link except to localhost.
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// message builds a plain-text UTF-8 email. Header values never carry CR/LF (no header injection).
func (s *Sender) message(to *mail.Address, d domain.Delivery) []byte {
	clean := func(v string) string { return strings.Join(strings.Fields(v), " ") }
	var id [12]byte
	_, _ = rand.Read(id[:])
	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", s.from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", clean(d.Subject)))
	header("Date", time.Now().UTC().Format(time.RFC1123Z))
	header("Message-Id", "<"+hex.EncodeToString(id[:])+"@"+domainOf(s.from.Address)+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	header("Auto-Submitted", "auto-generated") // RFC 3834: no auto-replies to notifications
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(d.Body, "\n", "\r\n")))
	_ = qp.Close()
	return b.Bytes()
}

func domainOf(addr string) string {
	if i := strings.LastIndexByte(addr, '@'); i >= 0 {
		return addr[i+1:]
	}
	return "kyber"
}
