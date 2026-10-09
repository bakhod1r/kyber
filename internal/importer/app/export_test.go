package app_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/app"
	"github.com/bakhod1r/kyber/internal/importer/domain"
)

type fakeSource []domain.ExportRow

func (f fakeSource) ExportRows(_ context.Context, project string) ([]domain.ExportRow, error) {
	return f, nil
}

func TestExport(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(app.Deps{Access: fakeAccess{"u-lead": "admin", "u-dev": "member", "u-view": "viewer"},
		Source: fakeSource{{Key: "KYB-1", Summary: "Login", Type: "bug", Status: "done", Priority: "high"}}})
	var buf bytes.Buffer
	if err := s.Export(ctx, "u-view", "KYB", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "KYB-1,Login,Bug,Done,Done,High") {
		t.Fatalf("csv = %q", buf.String())
	}
	if err := s.Export(ctx, "u-alien", "KYB", &buf); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
}
