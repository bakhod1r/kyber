package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/docs/domain/repotest"
	"github.com/bakhod1r/kyber/internal/docs/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }

func TestCommentsContract(t *testing.T) {
	pages := memory.NewRepository()
	comments := memory.NewComments(pages)
	repotest.RunComments(t, pages, comments)
	if len(comments.Outbox()) != 2 || len(comments.OutboxMeta()) != 2 || comments.Outbox()[0].EventName() != "doc.comment_added" {
		t.Fatalf("outbox = %+v", comments.Outbox())
	}
}
