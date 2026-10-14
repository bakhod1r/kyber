package livekit_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/call/application"
	"github.com/bakhod1r/kyber/internal/call/infrastructure/livekit"
)

// fakeLiveKit answers LiveKit's Twirp API (RoomService, Egress) like the server does: snake_case JSON, enum names,
// {"code","msg"} errors. It records each call with the grant of its token.
type fakeLiveKit struct {
	mu     sync.Mutex
	calls  []string
	status string // what ListEgress reports
	file   string // the egress-side file name it reports
}

func (f *fakeLiveKit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(tok, ".")
	var claims struct{ Video map[string]any }
	if len(parts) == 3 {
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(raw, &claims)
	}
	method := strings.TrimPrefix(r.URL.Path, "/twirp/livekit.")
	grant, _ := json.Marshal(claims.Video)
	in, _ := json.Marshal(body)
	f.mu.Lock()
	f.calls = append(f.calls, method+" "+string(in)+" "+string(grant))
	status, file := f.status, f.file
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "RoomService/RemoveParticipant":
		if body["identity"] == "gone" {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"code":"not_found","msg":"participant not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	case "Egress/StartRoomCompositeEgress":
		if body["room_name"] == "broken" {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"code":"unavailable","msg":"no response from servers"}`))
			return
		}
		_, _ = w.Write([]byte(`{"egress_id":"EG_1","room_name":"` + body["room_name"].(string) + `","status":"EGRESS_STARTING"}`))
	case "Egress/StopEgress":
		if body["egress_id"] == "EG_done" {
			w.WriteHeader(412)
			_, _ = w.Write([]byte(`{"code":"failed_precondition","msg":"egress already complete"}`))
			return
		}
		_, _ = w.Write([]byte(`{"egress_id":"` + body["egress_id"].(string) + `","status":"EGRESS_ENDING"}`))
	case "Egress/ListEgress":
		_, _ = w.Write([]byte(`{"items":[{"egress_id":"` + body["egress_id"].(string) + `","status":"` + status +
			`","file_results":[{"filename":"` + file + `","size":"5"}]}]}`))
	default:
		w.WriteHeader(404)
	}
}

func (f *fakeLiveKit) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newAPI(t *testing.T, fake *fakeLiveKit, dir string) *livekit.LiveKit {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	lk, err := livekit.New(livekit.Config{URL: "wss://sfu.example.com", APIURL: srv.URL, Key: "k", Secret: "s",
		RecordingsDir: dir, EgressDir: "/out", Wait: 300 * time.Millisecond, Poll: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return lk
}

func TestEvict(t *testing.T) {
	fake := &fakeLiveKit{}
	lk := newAPI(t, fake, "")
	ctx := context.Background()
	if err := lk.Evict(ctx, "call-1", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := lk.Evict(ctx, "call-1", "gone"); err != nil {
		t.Fatalf("already gone = %v", err)
	}
	want := `RoomService/RemoveParticipant {"identity":"user-1","room":"call-1"} {"room":"call-1","roomAdmin":true}`
	if l := fake.log(); len(l) != 2 || l[0] != want {
		t.Fatalf("calls = %v", l)
	}
}

func TestEgress(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeLiveKit{status: "EGRESS_ACTIVE"}
	lk := newAPI(t, fake, dir)
	ctx := context.Background()
	id, err := lk.Start(ctx, "call-1")
	if err != nil || id != "EG_1" {
		t.Fatalf("start = %q %v", id, err)
	}
	start := fake.log()[0]
	if !strings.HasPrefix(start, `Egress/StartRoomCompositeEgress {"file_outputs":[{"file_type":"MP4","filepath":"/out/call-1-`) ||
		!strings.Contains(start, `"layout":"grid","room_name":"call-1"} {"roomRecord":true}`) {
		t.Fatalf("start call = %s", start)
	}
	if _, err := lk.Start(ctx, "broken"); err == nil || !strings.Contains(err.Error(), "no response from servers") {
		t.Fatalf("egress down = %v", err)
	}
	if err := lk.Stop(ctx, "EG_1"); err != nil {
		t.Fatal(err)
	}
	if err := lk.Stop(ctx, "EG_done"); err != nil {
		t.Fatalf("stop finished = %v", err)
	}

	// Still being written: Open gives up after Wait so the relay retries later.
	if _, err := lk.Open(ctx, "EG_1"); err == nil || errors.Is(err, application.ErrRecordingLost) {
		t.Fatalf("pending = %v", err)
	}
	fake.mu.Lock()
	fake.status, fake.file = "EGRESS_COMPLETE", "/out/call-1-1.mp4"
	fake.mu.Unlock()
	if err := os.WriteFile(filepath.Join(dir, "call-1-1.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := lk.Open(ctx, "EG_1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	_ = f.Close()
	if string(b) != "video" {
		t.Fatalf("file = %q", b)
	}
	if err := lk.Discard(ctx, "EG_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "call-1-1.mp4")); !os.IsNotExist(err) {
		t.Fatalf("not discarded: %v", err)
	}
	if err := lk.Discard(ctx, "EG_1"); err != nil {
		t.Fatalf("discard twice = %v", err)
	}
	// A file name that is not a plain name in the egress directory is never opened.
	for _, bad := range []string{"/etc/passwd", "/out/../etc/passwd", "/out/sub/x.mp4"} {
		fake.mu.Lock()
		fake.file = bad
		fake.mu.Unlock()
		if _, err := lk.Open(ctx, "EG_1"); !errors.Is(err, application.ErrRecordingLost) {
			t.Fatalf("%s = %v", bad, err)
		}
	}
	for _, status := range []string{"EGRESS_FAILED", "EGRESS_ABORTED"} {
		fake.mu.Lock()
		fake.status = status
		fake.mu.Unlock()
		if _, err := lk.Open(ctx, "EG_1"); !errors.Is(err, application.ErrRecordingLost) {
			t.Fatalf("%s = %v", status, err)
		}
	}
	// LIMIT_REACHED still wrote a file.
	fake.mu.Lock()
	fake.status, fake.file = "EGRESS_LIMIT_REACHED", "/out/call-1-2.mp4"
	fake.mu.Unlock()
	_ = os.WriteFile(filepath.Join(dir, "call-1-2.mp4"), []byte("v"), 0o600)
	if f, err := lk.Open(ctx, "EG_1"); err != nil {
		t.Fatalf("limit reached = %v", err)
	} else {
		_ = f.Close()
	}
}

func TestEgressNeedsDirectory(t *testing.T) {
	lk, _ := livekit.New(livekit.Config{URL: "wss://sfu.example.com", Key: "k", Secret: "s"})
	if lk.Records() {
		t.Fatal("records without a recordings directory")
	}
	if !newAPI(t, &fakeLiveKit{}, t.TempDir()).Records() {
		t.Fatal("does not record with a directory")
	}
	if got := lk.APIBase(); got != "https://sfu.example.com" {
		t.Fatalf("api base = %s", got)
	}
}

func TestAPIFailures(t *testing.T) {
	ctx := context.Background()
	if _, err := livekit.New(livekit.Config{URL: "wss://x.example", APIURL: "ftp://x", Key: "k", Secret: "s"}); err == nil {
		t.Fatal("bad api url accepted")
	}
	// LiveKit unreachable.
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	down, _ := livekit.New(livekit.Config{URL: "wss://x.example", APIURL: srv.URL, Key: "k", Secret: "s", RecordingsDir: t.TempDir()})
	if err := down.Evict(ctx, "r", "u"); err == nil {
		t.Fatal("evict while down")
	}
	if _, err := down.Open(ctx, "EG_1"); err == nil {
		t.Fatal("open while down")
	}
	if err := down.Discard(ctx, "EG_1"); err == nil {
		t.Fatal("discard while down")
	}
	// Errors that are not Twirp JSON, and answers that are not JSON.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "ListEgress") {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "StartRoomCompositeEgress") {
			_, _ = w.Write([]byte(`not json`))
			return
		}
		w.WriteHeader(502)
	}))
	t.Cleanup(broken.Close)
	lk, _ := livekit.New(livekit.Config{URL: "wss://x.example", APIURL: broken.URL, Key: "k", Secret: "s", RecordingsDir: t.TempDir()})
	if err := lk.Stop(ctx, "EG_1"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("502 = %v", err)
	}
	if _, err := lk.Start(ctx, "r"); err == nil {
		t.Fatal("non-JSON answer accepted")
	}
	if _, err := lk.Open(ctx, "EG_gone"); !errors.Is(err, application.ErrRecordingLost) {
		t.Fatalf("unknown egress = %v", err)
	}
}

func TestOpenAndDiscardEdges(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeLiveKit{status: "EGRESS_COMPLETE", file: "/out/missing.mp4"}
	lk := newAPI(t, fake, dir)
	ctx := context.Background()
	if _, err := lk.Open(ctx, "EG_1"); !errors.Is(err, application.ErrRecordingLost) {
		t.Fatalf("file missing = %v", err)
	}
	fake.file = "/elsewhere/x.mp4"
	if err := lk.Discard(ctx, "EG_1"); !errors.Is(err, application.ErrRecordingLost) {
		t.Fatalf("discard outside = %v", err)
	}
	// A directory in its place cannot be removed as a file.
	fake.file = "/out/busy.mp4"
	_ = os.MkdirAll(filepath.Join(dir, "busy.mp4", "x"), 0o700)
	if err := lk.Discard(ctx, "EG_1"); err == nil {
		t.Fatal("removing a non-empty directory succeeded")
	}
	fake.status = "EGRESS_ACTIVE"
	cancelled, cancel := context.WithTimeout(ctx, 50*time.Millisecond) // ends while Open waits between polls
	defer cancel()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	waiting, _ := livekit.New(livekit.Config{URL: "wss://x.example", APIURL: srv.URL, Key: "k", Secret: "s", RecordingsDir: dir,
		Wait: time.Hour, Poll: time.Hour})
	if _, err := waiting.Open(cancelled, "EG_1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("open after cancel")
	}
}
