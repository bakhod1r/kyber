// Package domain is the Attachment context: files attached to issues. The bytes live in a blob
// store; this package keeps what is known about them.
package domain

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidAttachment  = errors.New("invalid attachment")
	ErrTooLarge           = errors.New("attachment is too large")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

// MaxSize bounds one file (25 MiB).
const MaxSize int64 = 25 << 20

const maxName = 255

// inline lists the types a browser may show in place; everything else (HTML, SVG, scripts)
// downloads, so an uploaded file can never run in Kyber's origin.
var inline = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "application/pdf": true}

type ID string

// Attachment is one file on an issue.
type Attachment struct {
	ID          ID
	Issue       string // issue key
	Name        string
	ContentType string
	Size        int64
	SHA256      string // hex
	UploadedBy  string
	UploadedAt  time.Time
}

// New describes an uploaded file. The name is reduced to a safe base name and an unparsable
// content type becomes application/octet-stream.
func New(id ID, issue, name, contentType string, size int64, sum, by string, at time.Time) (*Attachment, error) {
	name = cleanName(name)
	if name == "" || name == "." || name == ".." || utf8.RuneCountInString(name) > maxName {
		return nil, fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidAttachment, maxName)
	}
	if size <= 0 {
		return nil, fmt.Errorf("%w: file is empty", ErrInvalidAttachment)
	}
	if size > MaxSize {
		return nil, fmt.Errorf("%w: at most %d MiB", ErrTooLarge, MaxSize>>20)
	}
	return &Attachment{ID: id, Issue: issue, Name: name, ContentType: cleanType(contentType), Size: size, SHA256: sum,
		UploadedBy: by, UploadedAt: at}, nil
}

func cleanName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ReplaceAll(name, `\`, "/"))
	return strings.TrimSpace(path.Base(strings.TrimSpace(name)))
}

func cleanType(ct string) string {
	t, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return "application/octet-stream"
	}
	return mime.FormatMediaType(t, params)
}

// Project is the key of the project the issue belongs to.
func (a *Attachment) Project() string {
	p, _, _ := strings.Cut(a.Issue, "-")
	return p
}

// Inline reports whether browsers may display the file in place.
func (a *Attachment) Inline() bool {
	t, _, _ := mime.ParseMediaType(a.ContentType)
	return inline[t]
}

// MayDelete: the uploader, or someone who manages the project's attachments.
func (a *Attachment) MayDelete(actor string, manager bool) bool {
	return manager || a.UploadedBy == actor
}

// Repository stores attachment records in the workspace of ctx.
type Repository interface {
	Save(ctx context.Context, a *Attachment) error
	ByID(ctx context.Context, id ID) (*Attachment, error)
	// ForIssue lists an issue's attachments, oldest first.
	ForIssue(ctx context.Context, issue string) ([]*Attachment, error)
	Delete(ctx context.Context, id ID) error
}
