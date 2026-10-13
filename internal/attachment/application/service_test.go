package application_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/attachment/application"
	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/memory"
)

// issues: ann edits KYB, ben only views it, max manages it; KYB-9 is hidden from everyone.
type issues struct{}

func (issues) Authorize(_ context.Context, actor, issue string, need application.Need) (string, error) {
	if issue == "KYB-9" || actor == "out" {
		return "", application.ErrIssueNotFound
	}
	allowed := map[string]application.Need{"ann": application.Attach, "ben": application.View, "max": application.Manage}
	if need > allowed[actor] {
		return "", application.ErrForbidden
	}
	return strings.ToUpper(issue), nil
}

type brokenBlobs struct {
	*memory.Blobs
	put, open bool
}

func (b brokenBlobs) Put(ctx context.Context, id domain.ID, r io.Reader) (int64, string, error) {
	if b.put {
		return 0, "", errors.New("disk full")
	}
	return b.Blobs.Put(ctx, id, r)
}

func (b brokenBlobs) Open(ctx context.Context, id domain.ID) (io.ReadCloser, error) {
	if b.open {
		return nil, errors.New("disk gone")
	}
	return b.Blobs.Open(ctx, id)
}

type brokenRepo struct{ *memory.Repository }

func (brokenRepo) Save(context.Context, *domain.Attachment) error { return errors.New("db down") }

func setup(blobs application.Blobs, repo domain.Repository) *application.Service {
	n := 0
	return application.NewService(application.Deps{Issues: issues{}, Attachments: repo, Blobs: blobs,
		NewID: func() string { n++; return "94000000-0000-4000-8000-00000000000" + string(rune('0'+n)) },
		Now:   func() time.Time { return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC) }})
}

func TestAttachments(t *testing.T) {
	ctx := context.Background()
	blobs := memory.NewBlobs()
	svc := setup(blobs, memory.NewRepository())

	a, err := svc.Upload(ctx, "ann", "kyb-1", "log.txt", "text/plain", strings.NewReader("boom"))
	if err != nil || a.Issue != "KYB-1" || a.Size != 4 || a.UploadedBy != "ann" {
		t.Fatalf("upload = %+v, %v", a, err)
	}
	if _, err := svc.Upload(ctx, "ben", "KYB-1", "x.txt", "text/plain", strings.NewReader("x")); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer upload: %v", err)
	}
	if _, err := svc.Upload(ctx, "ann", "KYB-1", "empty.txt", "text/plain", strings.NewReader("")); !errors.Is(err, domain.ErrInvalidAttachment) {
		t.Fatalf("empty upload: %v", err)
	}
	big := io.LimitReader(zeros{}, domain.MaxSize+10)
	if _, err := svc.Upload(ctx, "ann", "KYB-1", "big.bin", "", big); !errors.Is(err, domain.ErrTooLarge) {
		t.Fatalf("big upload: %v", err)
	}

	list, err := svc.List(ctx, "ben", "KYB-1")
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list = %v, %v", list, err)
	}
	if _, err := svc.List(ctx, "out", "KYB-1"); !errors.Is(err, application.ErrIssueNotFound) {
		t.Fatalf("outsider list: %v", err)
	}

	got, r, err := svc.Open(ctx, "ben", string(a.ID))
	if err != nil || got.Name != "log.txt" {
		t.Fatalf("open = %v, %v", got, err)
	}
	data, _ := io.ReadAll(r)
	r.Close()
	if string(data) != "boom" {
		t.Fatalf("contents %q", data)
	}
	if _, _, err := svc.Open(ctx, "out", string(a.ID)); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("outsider open: %v", err)
	}
	if _, _, err := svc.Open(ctx, "ann", "nope"); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("missing open: %v", err)
	}

	if err := svc.Delete(ctx, "ben", string(a.ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer delete: %v", err)
	}
	if err := svc.Delete(ctx, "out", string(a.ID)); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("outsider delete: %v", err)
	}
	if err := svc.Delete(ctx, "ann", string(a.ID)); err != nil {
		t.Fatal(err)
	}
	b, _ := svc.Upload(ctx, "ann", "KYB-1", "b.txt", "text/plain", strings.NewReader("b"))
	if err := svc.Delete(ctx, "max", string(b.ID)); err != nil {
		t.Fatalf("manager delete: %v", err)
	}
	if err := svc.Delete(ctx, "max", string(b.ID)); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

// A file hidden behind a forbidden (not missing) issue reports the real error.
func TestOpenForbidden(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRepository()
	a, _ := domain.New("94000000-0000-4000-8000-000000000009", "KYB-1", "a.txt", "text/plain", 1, "s", "ann", time.Now())
	_ = repo.Save(ctx, a)
	svc := application.NewService(application.Deps{Issues: forbid{}, Attachments: repo, Blobs: memory.NewBlobs()})
	if _, _, err := svc.Open(ctx, "ann", string(a.ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("open: %v", err)
	}
}

type forbid struct{}

func (forbid) Authorize(context.Context, string, string, application.Need) (string, error) {
	return "", application.ErrForbidden
}

func TestStorageFailures(t *testing.T) {
	ctx := context.Background()
	if _, err := setup(brokenBlobs{Blobs: memory.NewBlobs(), put: true}, memory.NewRepository()).
		Upload(ctx, "ann", "KYB-1", "a.txt", "text/plain", strings.NewReader("a")); err == nil {
		t.Fatal("put failure ignored")
	}
	blobs := memory.NewBlobs()
	if _, err := setup(blobs, brokenRepo{memory.NewRepository()}).
		Upload(ctx, "ann", "KYB-1", "a.txt", "text/plain", strings.NewReader("a")); err == nil {
		t.Fatal("save failure ignored")
	}
	if _, err := blobs.Open(ctx, "94000000-0000-4000-8000-000000000001"); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("blob left behind after a failed save: %v", err)
	}
	broken := brokenBlobs{Blobs: memory.NewBlobs(), open: true}
	svc := setup(broken, memory.NewRepository())
	a, _ := svc.Upload(ctx, "ann", "KYB-1", "a.txt", "text/plain", strings.NewReader("a"))
	if _, _, err := svc.Open(ctx, "ann", string(a.ID)); err == nil {
		t.Fatal("open failure ignored")
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type undeletable struct{ *memory.Repository }

func (undeletable) Delete(context.Context, domain.ID) error { return errors.New("db down") }

func TestDeleteFailure(t *testing.T) {
	ctx := context.Background()
	svc := setup(memory.NewBlobs(), undeletable{memory.NewRepository()})
	a, _ := svc.Upload(ctx, "ann", "KYB-1", "a.txt", "text/plain", strings.NewReader("a"))
	if err := svc.Delete(ctx, "ann", string(a.ID)); err == nil {
		t.Fatal("delete failure ignored")
	}
}
