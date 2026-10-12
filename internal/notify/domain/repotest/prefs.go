package repotest

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func RunPrefs(t *testing.T, newRepo func(t *testing.T) domain.PrefsRepository) {
	ctx := context.Background()
	repo := newRepo(t)
	if p, err := repo.Prefs(ctx, Alice); err != nil || p != domain.DefaultPrefs() {
		t.Fatalf("default = %+v, %v", p, err)
	}
	for _, want := range []domain.Prefs{{Email: false, Telegram: true}, {Email: true, Telegram: false}} {
		if err := repo.SetPrefs(ctx, Alice, want); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.Prefs(ctx, Alice); got != want {
			t.Fatalf("prefs = %+v, want %+v", got, want)
		}
	}
	if p, _ := repo.Prefs(ctx, Bob); p != domain.DefaultPrefs() {
		t.Fatalf("other user affected: %+v", p)
	}
}
