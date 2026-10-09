package guardlimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/bakhod1r/kyber/internal/identity/adapter/guardlimit"
	"github.com/bakhod1r/kyber/internal/identity/app"
)

func TestSharedAcrossInstances(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()
	// Two "replicas" with their own clients share one Redis.
	a := guardlimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "kyber:")
	b := guardlimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "kyber:")

	for i := range app.LoginAttempts {
		l := a
		if i%2 == 1 {
			l = b
		}
		if ok, _, err := l.Allow(ctx, "login:ali@x.uz|ip"); err != nil || !ok {
			t.Fatalf("attempt %d: allowed=%v err=%v", i+1, ok, err)
		}
	}
	ok, retry, err := b.Allow(ctx, "login:ali@x.uz|ip")
	if err != nil || ok || retry <= 0 || retry > app.LoginWindow {
		t.Fatalf("over limit: allowed=%v retry=%v err=%v", ok, retry, err)
	}
	if ok, _, _ := a.Allow(ctx, "login:ali@x.uz|other-ip"); !ok {
		t.Fatal("other key must not be limited")
	}
}

func TestRedisDownIsAnError(t *testing.T) {
	mr := miniredis.RunT(t)
	l := guardlimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 200 * time.Millisecond}), "kyber:")
	mr.Close()
	if _, _, err := l.Allow(context.Background(), "k"); err == nil {
		t.Fatal("unreachable Redis must surface an error (the service fails closed)")
	}
}
