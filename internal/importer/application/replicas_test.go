package application_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/importer/application"
	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/infrastructure/memory"
)

// slowIssues is an Issue Tracking port shared by replicas; each import takes a moment, so runs overlap.
type slowIssues struct {
	mu sync.Mutex
	n  int
}

func (f *slowIssues) Import(_ context.Context, project, _ string, _ domain.Item) (string, error) {
	time.Sleep(20 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return fmt.Sprintf("%s-%d", project, f.n), nil
}

// QA-08: two replicas importing the same file at the same moment must not create an issue twice for one row,
// and neither run fails; the second waits for the first and skips what it imported.
func TestTwoReplicasImportingTheSameFileCreateEachIssueOnce(t *testing.T) {
	issues, maps, lock := &slowIssues{}, memory.NewRepository(), memory.NewImportLock()
	replica := func() *application.Service {
		return application.NewService(application.Deps{Access: fakeAccess{"u-lead": "admin"}, Members: fakeAccess{},
			Issues: issues, Mappings: maps, Lock: lock})
	}
	a, b := replica(), replica()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, s := range []*application.Service{a, b} {
		wg.Go(func() { _, errs[i] = s.Import(context.Background(), "u-lead", "KYB", csv, false) })
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("errors = %v", errs)
	}
	if issues.n != 2 {
		t.Fatalf("created %d issues for 2 rows", issues.n)
	}
}
