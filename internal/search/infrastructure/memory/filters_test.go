package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/search/domain/repotest"
	"github.com/bakhod1r/kyber/internal/search/infrastructure/memory"
)

func TestFilterContract(t *testing.T) { repotest.RunFilters(t, memory.NewFilterRepository()) }
