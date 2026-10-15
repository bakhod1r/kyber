package repotest

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func RunPrefs(t *testing.T, newRepo func(t *testing.T) domain.PrefsRepository) {
	ctx := context.Background()
	repo := newRepo(t)
	if p, err := repo.Prefs(ctx, Alice); err != nil || p != domain.DefaultPrefs() {
		t.Fatalf("default = %+v, %v", p, err)
	}
	for _, want := range []domain.Prefs{{Email: false, Telegram: true}, {Email: true, Telegram: false, TimeZone: "Asia/Tashkent"}, {Email: true, Telegram: true}} {
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

	// B9: the daily digest is a preference; subscribers are listed with when their last digest went out.
	if subs, err := repo.DigestSubscribers(ctx); err != nil || len(subs) != 0 {
		t.Fatalf("subscribers before = %v, %v", subs, err)
	}
	digest := domain.Prefs{Email: true, Telegram: true, TimeZone: "Asia/Tashkent", EmailDigest: true}
	if err := repo.SetPrefs(ctx, Alice, digest); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Prefs(ctx, Alice); got != digest {
		t.Fatalf("digest prefs = %+v", got)
	}
	subs, err := repo.DigestSubscribers(ctx)
	if err != nil || len(subs) != 1 || subs[0].User != Alice || subs[0].Prefs != digest || !subs[0].LastSent.IsZero() {
		t.Fatalf("subscribers = %+v, %v", subs, err)
	}
	at := time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	if err := repo.MarkDigestSent(ctx, Alice, at); err != nil {
		t.Fatal(err)
	}
	if subs, _ := repo.DigestSubscribers(ctx); len(subs) != 1 || !subs[0].LastSent.Equal(at) {
		t.Fatalf("after send = %+v", subs)
	}
	// Changing other preferences keeps the last send time.
	digest.TimeZone = ""
	_ = repo.SetPrefs(ctx, Alice, digest)
	if subs, _ := repo.DigestSubscribers(ctx); len(subs) != 1 || !subs[0].LastSent.Equal(at) {
		t.Fatalf("after prefs change = %+v", subs)
	}
	digest.EmailDigest = false
	_ = repo.SetPrefs(ctx, Alice, digest)
	if subs, _ := repo.DigestSubscribers(ctx); len(subs) != 0 {
		t.Fatalf("after opting out = %+v", subs)
	}
}
