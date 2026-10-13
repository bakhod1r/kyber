package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func TestNew(t *testing.T) {
	tests := []struct {
		name, file, ctype  string
		size               int64
		wantName, wantType string
		inline             bool
		err                error
	}{
		{"image", "shot.png", "image/png", 10, "shot.png", "image/png", true, nil},
		{"path parts dropped", `..\..\etc/passwd`, "text/plain; charset=utf-8", 1, "passwd", "text/plain; charset=utf-8", false, nil},
		{"control characters dropped", "a\x00b\r\n.txt", "text/plain", 1, "ab.txt", "text/plain", false, nil},
		{"bad type is octet-stream", "x.bin", "not a type", 1, "x.bin", "application/octet-stream", false, nil},
		{"empty type is octet-stream", "x.bin", "", 1, "x.bin", "application/octet-stream", false, nil},
		{"svg never inline", "x.svg", "image/svg+xml", 1, "x.svg", "image/svg+xml", false, nil},
		{"html never inline", "x.html", "text/html", 1, "x.html", "text/html", false, nil},
		{"empty name", "  ", "text/plain", 1, "", "", false, domain.ErrInvalidAttachment},
		{"dot name", "..", "text/plain", 1, "", "", false, domain.ErrInvalidAttachment},
		{"long name", strings.Repeat("a", 256), "text/plain", 1, "", "", false, domain.ErrInvalidAttachment},
		{"empty file", "a.txt", "text/plain", 0, "", "", false, domain.ErrInvalidAttachment},
		{"too large", "a.txt", "text/plain", domain.MaxSize + 1, "", "", false, domain.ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := domain.New("id", "KYB-1", tt.file, tt.ctype, tt.size, "sum", "ann", t0)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if err != nil {
				return
			}
			if a.Name != tt.wantName || a.ContentType != tt.wantType || a.Inline() != tt.inline || a.Project() != "KYB" || a.Size != tt.size {
				t.Fatalf("attachment = %+v inline=%v", a, a.Inline())
			}
		})
	}
}

func TestMayDelete(t *testing.T) {
	a, _ := domain.New("id", "KYB-1", "a.txt", "text/plain", 1, "sum", "ann", t0)
	if !a.MayDelete("ann", false) || a.MayDelete("ben", false) || !a.MayDelete("ben", true) {
		t.Fatal("only the uploader or a manager may delete")
	}
}
