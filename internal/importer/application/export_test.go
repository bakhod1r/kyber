package application_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/application"
	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/infrastructure/memory"
)

type fakeSource []domain.ExportRow

func (f fakeSource) ExportRows(_ context.Context, _, project string) ([]domain.ExportRow, error) {
	return f, nil
}

func TestExport(t *testing.T) {
	ctx := context.Background()
	s := application.NewService(application.Deps{Access: fakeAccess{"u-lead": "admin", "u-dev": "member", "u-view": "viewer"},
		Source: fakeSource{{Key: "KYB-1", Summary: "Login", Type: "bug", Status: "done", Priority: "high"}}})
	var buf bytes.Buffer
	if err := s.Export(ctx, "u-view", "KYB", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "KYB-1,Login,Bug,Done,Done,High") {
		t.Fatalf("csv = %q", buf.String())
	}
	if err := s.Export(ctx, "u-alien", "KYB", &buf); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
}

// QA parity: importing a project's own export into it skips the issues it already has.
func TestImportSkipsIssuesAlreadyInTheProject(t *testing.T) {
	issues, maps := &fakeIssues{}, memory.NewRepository()
	s := application.NewService(application.Deps{Access: fakeAccess{"u-lead": "admin"}, Members: fakeAccess{}, Issues: issues, Mappings: maps, Lock: memory.NewImportLock(),
		Source: fakeSource{{Key: "KYB-1", Summary: "Login", Type: "bug", Status: "done", Priority: "high"}}})
	r, err := s.Import(context.Background(), "u-lead", "KYB", "Summary,Issue key\nLogin,KYB-1\nNew,PROJ-9\n", false)
	if err != nil || len(r.Items) != 1 || r.Items[0].Key != "PROJ-9" || len(r.Skipped) != 1 || r.Skipped[0] != "KYB-1" {
		t.Fatalf("report = %+v %v", r, err)
	}
}

type brokenSource struct{}

func (brokenSource) ExportRows(context.Context, string, string) ([]domain.ExportRow, error) {
	return nil, errors.New("db down")
}

func TestImportFailsWhenOwnIssuesCannotBeRead(t *testing.T) {
	s := application.NewService(application.Deps{Access: fakeAccess{"u-lead": "admin"}, Members: fakeAccess{}, Issues: &fakeIssues{},
		Mappings: memory.NewRepository(), Source: brokenSource{}})
	if _, err := s.Import(context.Background(), "u-lead", "KYB", "Summary,Issue key\nx,P-1\n", true); err == nil {
		t.Fatal("expected the source error")
	}
}
