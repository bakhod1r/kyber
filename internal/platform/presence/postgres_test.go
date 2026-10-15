package presence_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/presence"
	"github.com/bakhod1r/kyber/internal/platform/presence/presencetest"
)

func TestPostgresContract(t *testing.T) { presencetest.Run(t, presence.NewPostgres(dbtest.New(t))) }
