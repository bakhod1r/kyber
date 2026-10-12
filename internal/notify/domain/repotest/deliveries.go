package repotest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func dl(t *testing.T, n int, key string, at time.Time) domain.Delivery {
	t.Helper()
	d, err := domain.NewDelivery(uuidFor(n), "00000000-0000-4000-8000-000000000001", domain.ChannelEmail, "a@x.uz", "Subject", "Body", key, at)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func uuidFor(n int) string { return "d0000000-0000-4000-8000-" + pad(n) }

func pad(n int) string {
	s := "000000000000"
	d := []byte(s)
	for i := len(d) - 1; n > 0 && i >= 0; i-- {
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d)
}

func RunDeliveries(t *testing.T, newRepo func(t *testing.T) domain.DeliveryRepository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	t.Run("enqueue is idempotent; claim returns due ones oldest first", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.Enqueue(ctx, dl(t, 2, "n-2/email", t0.Add(time.Second)))
		_ = repo.Enqueue(ctx, dl(t, 1, "n-1/email", t0))
		_ = repo.Enqueue(ctx, dl(t, 3, "n-1/email", t0)) // same key: ignored
		_ = repo.Enqueue(ctx, dl(t, 4, "n-4/email", t0.Add(time.Hour)))
		got, err := repo.Claim(ctx, t0.Add(2*time.Second), time.Minute, 10)
		if err != nil || len(got) != 2 || got[0].ID != uuidFor(1) || got[1].ID != uuidFor(2) {
			t.Fatalf("claimed = %+v, %v", got, err)
		}
		if got[0].To != "a@x.uz" || got[0].Subject != "Subject" || got[0].Body != "Body" || got[0].Channel != domain.ChannelEmail {
			t.Fatalf("values = %+v", got[0])
		}
		// Claimed ones are leased: not handed out again until the lease ends.
		if again, _ := repo.Claim(ctx, t0.Add(3*time.Second), time.Minute, 10); len(again) != 0 {
			t.Fatalf("leased handed out again: %+v", again)
		}
		if after, _ := repo.Claim(ctx, t0.Add(2*time.Minute), time.Minute, 10); len(after) != 2 {
			t.Fatalf("expired lease not reclaimed: %+v", after)
		}
	})
	t.Run("save records outcomes", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.Enqueue(ctx, dl(t, 1, "a", t0))
		_ = repo.Enqueue(ctx, dl(t, 2, "b", t0))
		got, _ := repo.Claim(ctx, t0, time.Minute, 10)
		got[0].Sent(t0)
		got[1].Failed("451", false, t0) // retry in 30 s
		for _, d := range got {
			if err := repo.Save(ctx, d); err != nil {
				t.Fatal(err)
			}
		}
		later, _ := repo.Claim(ctx, t0.Add(31*time.Second), time.Minute, 10)
		if len(later) != 1 || later[0].ID != uuidFor(2) || later[0].Attempts != 1 || later[0].LastError != "451" {
			t.Fatalf("retry = %+v", later)
		}
		later[0].Failed("550 no such user", true, t0)
		_ = repo.Save(ctx, later[0])
		if none, _ := repo.Claim(ctx, t0.Add(48*time.Hour), time.Minute, 10); len(none) != 0 {
			t.Fatalf("sent/dead deliveries must not be claimed: %+v", none)
		}
	})
	t.Run("concurrent dispatchers never share a delivery", func(t *testing.T) {
		repo := newRepo(t)
		for n := 1; n <= 20; n++ {
			_ = repo.Enqueue(ctx, dl(t, n, pad(n), t0))
		}
		var mu sync.Mutex
		seen := map[string]int{}
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := repo.Claim(ctx, t0, time.Minute, 7)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				for _, d := range got {
					seen[d.ID]++
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
		for id, n := range seen {
			if n > 1 {
				t.Fatalf("%s claimed %d times", id, n)
			}
		}
		if len(seen) != 20 {
			t.Fatalf("claimed %d of 20", len(seen))
		}
	})
}
