package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	app "github.com/bakhod1r/kyber/internal/call/application"
	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/call/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// convs is a fake Chat port: conversation → members; "pub" is a public channel qa can read but did not join.
type convs map[string][]string

func (c convs) Members(_ context.Context, actor, conv string) ([]string, error) {
	members, ok := c[conv]
	if !ok {
		return nil, app.ErrNoConversation
	}
	for _, m := range members {
		if m == actor {
			return members, nil
		}
	}
	if conv == "pub" {
		return nil, app.ErrNotMember
	}
	return nil, app.ErrNoConversation
}

type told []string

func (t *told) Notify(_ context.Context, to []string, c *domain.Call) {
	*t = append(*t, fmt.Sprintf("%v:%d", to, len(c.Participants)))
}

func (t *told) Follow(context.Context, []string, *domain.Call, app.View) {}

func (t *told) React(_ context.Context, to []string, _ *domain.Call, from, emoji string) {
	*t = append(*t, fmt.Sprintf("react %s %s→%v", from, emoji, to))
}

func setup() (*app.Service, context.Context, *told) {
	n := 0
	tl := &told{}
	svc := app.NewService(app.Deps{
		Calls: memory.NewRepository(), Conversations: convs{"dev": {"lead", "dev"}, "pub": {"lead"}},
		Notifier: tl, NewID: func() string { n++; return fmt.Sprintf("k-%d", n) }, Now: func() time.Time { return time.Unix(0, 0) },
	})
	return svc, tenant.With(context.Background(), "w1"), tl
}

func TestHuddle(t *testing.T) {
	svc, ctx, tl := setup()
	c, err := svc.Join(ctx, "lead", "dev")
	if err != nil || c.StartedBy != "lead" || len(c.Participants) != 1 || c.Workspace != "w1" {
		t.Fatalf("start = %+v %v", c, err)
	}
	if live, err := svc.Live(ctx, "dev", "dev"); err != nil || live == nil || live.ID != c.ID {
		t.Fatalf("live = %+v %v", live, err)
	}
	c2, _ := svc.Join(ctx, "dev", "dev")
	if c2.ID != c.ID || len(c2.Participants) != 2 {
		t.Fatalf("join = %+v", c2)
	}
	if _, err := svc.Join(ctx, "qa", "pub"); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("not joined the channel = %v", err)
	}
	if _, err := svc.Join(ctx, "eve", "dev"); !errors.Is(err, app.ErrNoConversation) {
		t.Fatalf("outsider = %v", err)
	}
	if _, err := svc.SetMedia(ctx, "eve", string(c.ID), domain.Media{Video: true}); !errors.Is(err, domain.ErrCallNotFound) {
		t.Fatalf("outsider media = %v (the call looks missing)", err)
	}
	m, err := svc.SetMedia(ctx, "dev", string(c.ID), domain.Media{Audio: true, Screen: true})
	if err != nil || !m.Participants[1].Media.Screen {
		t.Fatalf("media = %+v %v", m, err)
	}
	_ = svc.Leave(ctx, "lead", string(c.ID))
	_ = svc.Leave(ctx, "dev", string(c.ID))
	if live, err := svc.Live(ctx, "dev", "dev"); err != nil || live != nil {
		t.Fatalf("after everyone left = %+v %v", live, err)
	}
	if err := svc.Leave(ctx, "dev", string(c.ID)); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("leave twice = %v", err)
	}
	want := "[[lead dev]:1 [lead dev]:2 [lead dev]:2 [lead dev]:1 [lead dev]:0]"
	if fmt.Sprint(*tl) != want {
		t.Fatalf("told = %v, want %v", *tl, want)
	}
}

// racy is a repository whose first Save loses to another writer, to prove the retry.
type racy struct {
	*memory.Repository
	lost bool
}

func (r *racy) Save(ctx context.Context, c *domain.Call) error {
	if !r.lost {
		r.lost = true
		return domain.ErrConflict
	}
	return r.Repository.Save(ctx, c)
}

func TestJoinRetriesOnConflict(t *testing.T) {
	repo := &racy{Repository: memory.NewRepository()}
	svc := app.NewService(app.Deps{Calls: repo, Conversations: convs{"dev": {"lead", "dev"}}, NewID: func() string { return "k-1" }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	_, _ = svc.Join(ctx, "lead", "dev")
	if c, err := svc.Join(ctx, "dev", "dev"); err != nil || len(c.Participants) != 2 {
		t.Fatalf("join after conflict = %+v %v", c, err)
	}
}

type relayed []domain.Signal

func (r *relayed) Relay(_ context.Context, s domain.Signal) { *r = append(*r, s) }

func TestSignalRelay(t *testing.T) {
	r := &relayed{}
	svc := app.NewService(app.Deps{Calls: memory.NewRepository(), Conversations: convs{"dev": {"lead", "dev"}}, Signals: r,
		NewID: func() string { return "k-1" }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	c, _ := svc.Join(ctx, "lead", "dev")
	if err := svc.Signal(ctx, "lead", string(c.ID), "dev", "offer", "sdp"); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("to someone not in the call yet = %v", err)
	}
	_, _ = svc.Join(ctx, "dev", "dev")
	if err := svc.Signal(ctx, "lead", string(c.ID), "dev", "offer", "sdp"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Signal(ctx, "eve", string(c.ID), "dev", "offer", "sdp"); !errors.Is(err, domain.ErrCallNotFound) {
		t.Fatalf("outsider = %v", err)
	}
	if len(*r) != 1 || (*r)[0].To != "dev" || (*r)[0].Data != "sdp" {
		t.Fatalf("relayed = %+v", *r)
	}
}

// S75: raise a hand (stored on the participant) and send reactions to the others (not stored).
func TestHandsAndReactions(t *testing.T) {
	svc, ctx, tl := setup()
	c, _ := svc.Join(ctx, "lead", "dev")
	_, _ = svc.Join(ctx, "dev", "dev")
	got, err := svc.RaiseHand(ctx, "dev", string(c.ID), true)
	if err != nil || !got.Participants[1].Hand {
		t.Fatalf("raise = %+v %v", got, err)
	}
	*tl = nil
	if err := svc.React(ctx, "dev", string(c.ID), "🎉"); err != nil {
		t.Fatal(err)
	}
	if len(*tl) != 1 || (*tl)[0] != "react dev 🎉→[lead]" {
		t.Fatalf("told = %v", *tl)
	}
	if err := svc.React(ctx, "qa", string(c.ID), "🎉"); err == nil {
		t.Fatal("someone outside the call reacted")
	}
}

// rooms is a fake media server: it hands out a ticket naming the room and the user, and records evictions.
type rooms struct {
	fail    error
	evicted *[]string
}

func (r rooms) Evict(_ context.Context, room, user string) error {
	if r.evicted != nil {
		*r.evicted = append(*r.evicted, room+"/"+user)
	}
	return r.fail
}

func (r rooms) Ticket(room, user string, now time.Time) (app.RoomTicket, error) {
	return app.RoomTicket{URL: "wss://sfu.test", Token: room + "/" + user, Expires: now.Add(time.Hour)}, r.fail
}

func TestRoomCalls(t *testing.T) {
	_, ctx, _ := setup()
	n := 0
	deps := app.Deps{Calls: memory.NewRepository(), Conversations: convs{"dev": {"lead", "dev"}},
		NewID: func() string { n++; return fmt.Sprintf("k-%d", n) }, Now: func() time.Time { return time.Unix(0, 0) }}
	mesh := app.NewService(deps)
	c, _ := mesh.Join(ctx, "lead", "dev")
	if c.Room {
		t.Fatal("without a media server calls are a mesh")
	}
	if _, err := mesh.EnterRoom(ctx, "lead", string(c.ID)); !errors.Is(err, domain.ErrNotRoom) {
		t.Fatalf("mesh room = %v", err)
	}
	_ = mesh.Leave(ctx, "lead", string(c.ID))

	deps.Rooms = rooms{}
	svc := app.NewService(deps)
	c, _ = svc.Join(ctx, "lead", "dev")
	if !c.Room {
		t.Fatal("with a media server new calls are rooms")
	}
	tk, err := svc.EnterRoom(ctx, "lead", string(c.ID))
	if err != nil || tk.URL != "wss://sfu.test" || tk.Token != string(c.ID)+"/lead" || !tk.Expires.Equal(time.Unix(3600, 0)) {
		t.Fatalf("ticket = %+v %v", tk, err)
	}
	if _, err := svc.EnterRoom(ctx, "dev", string(c.ID)); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("member not in the call = %v", err)
	}
	if _, err := svc.EnterRoom(ctx, "qa", string(c.ID)); !errors.Is(err, domain.ErrCallNotFound) {
		t.Fatalf("outsider = %v", err)
	}
	broken := errors.New("no signing key")
	deps.Rooms = rooms{fail: broken}
	if _, err := app.NewService(deps).EnterRoom(ctx, "lead", string(c.ID)); !errors.Is(err, broken) {
		t.Fatalf("ticket failure = %v", err)
	}
	// The media server was turned off while a room call is live: nobody can enter it.
	deps.Rooms = nil
	if _, err := app.NewService(deps).EnterRoom(ctx, "lead", string(c.ID)); !errors.Is(err, app.ErrNoMediaServer) {
		t.Fatalf("media server gone = %v", err)
	}
}

// recorder is a fake LiveKit Egress: recordings are ids; a finished one opens as its id's bytes.
type recorder struct {
	log            []string
	startErr, open error
}

func (r *recorder) Start(_ context.Context, room string) (string, error) {
	r.log = append(r.log, "start "+room)
	return "EG_" + room, r.startErr
}
func (r *recorder) Stop(_ context.Context, egress string) error {
	r.log = append(r.log, "stop "+egress)
	return nil
}
func (r *recorder) Open(_ context.Context, egress string) (io.ReadCloser, error) {
	r.log = append(r.log, "open "+egress)
	return io.NopCloser(strings.NewReader("video of " + egress)), r.open
}
func (r *recorder) Discard(_ context.Context, egress string) error {
	r.log = append(r.log, "discard "+egress)
	return nil
}

type posted []string

func (p *posted) Post(_ context.Context, by, conv, name string, body io.Reader) error {
	b, _ := io.ReadAll(body)
	*p = append(*p, fmt.Sprintf("%s %s %q %s", by, conv, name, b))
	return nil
}

func roomService(t *testing.T) (*app.Service, *memory.Repository, *recorder, *posted, *[]string, context.Context) {
	t.Helper()
	_, ctx, _ := setup()
	n := 0
	repo, rec, out, evicted := memory.NewRepository(), &recorder{}, &posted{}, &[]string{}
	svc := app.NewService(app.Deps{Calls: repo, Conversations: convs{"dev": {"lead", "dev", "qa"}},
		Rooms: rooms{evicted: evicted}, Recorder: rec, Recordings: out,
		NewID: func() string { n++; return fmt.Sprintf("k-%d", n) }, Now: func() time.Time { return time.Date(2026, 10, 14, 9, 5, 0, 0, time.UTC) }})
	return svc, repo, rec, out, evicted, ctx
}

// relay delivers the events written so far (from index from) to the service's handlers, like the outbox relay.
func relay(t *testing.T, ctx context.Context, svc *app.Service, repo *memory.Repository, from int) int {
	t.Helper()
	events := repo.Outbox()
	for _, e := range events[from:] {
		payload, _ := json.Marshal(e)
		m := outbox.Message{Name: e.EventName(), Workspace: "w1", Payload: payload}
		h := map[string]func(context.Context, outbox.Message) error{
			"call.participant_left": svc.OnParticipantLeft, "call.recording_stopped": svc.OnRecordingStopped}[m.Name]
		if h == nil {
			continue
		}
		if err := h(ctx, m); err != nil {
			t.Fatalf("%s: %v", m.Name, err)
		}
	}
	return len(events)
}

func TestRemoveFromRoom(t *testing.T) {
	svc, repo, _, _, evicted, ctx := roomService(t)
	c, _ := svc.Join(ctx, "lead", "dev")
	_, _ = svc.Join(ctx, "dev", "dev")
	_, _ = svc.Join(ctx, "qa", "dev")
	if _, err := svc.Remove(ctx, "dev", string(c.ID), "qa"); !errors.Is(err, domain.ErrNotHost) {
		t.Fatalf("non-host = %v", err)
	}
	got, err := svc.Remove(ctx, "lead", string(c.ID), "qa")
	if err != nil || got.In("qa") {
		t.Fatalf("remove = %+v %v", got, err)
	}
	_ = svc.Leave(ctx, "dev", string(c.ID))
	relay(t, ctx, svc, repo, 0)
	if fmt.Sprint(*evicted) != "[k-1/qa k-1/dev]" {
		t.Fatalf("evicted = %v", *evicted)
	}
	// Without a media server (turned off meanwhile) there is no one to tell.
	payload, _ := json.Marshal(domain.ParticipantLeft{Call: "k-1", User: "lead"})
	if err := app.NewService(app.Deps{Calls: repo}).OnParticipantLeft(ctx, outbox.Message{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err := svc.OnParticipantLeft(ctx, outbox.Message{Payload: []byte("{bad")}); err != nil {
		t.Fatalf("poison = %v", err)
	}
}

func TestServerRecordingFlow(t *testing.T) {
	svc, repo, rec, out, _, ctx := roomService(t)
	c, _ := svc.Join(ctx, "lead", "dev")
	_, _ = svc.Join(ctx, "dev", "dev")
	got, err := svc.StartRecording(ctx, "dev", string(c.ID))
	if err != nil || got.Recording == nil || got.Recording.Egress != "EG_k-1" || got.Recording.By != "dev" {
		t.Fatalf("start = %+v %v", got, err)
	}
	if _, err := svc.StartRecording(ctx, "lead", string(c.ID)); !errors.Is(err, domain.ErrAlreadyRecording) {
		t.Fatalf("twice = %v", err)
	}
	if _, err := svc.StartRecording(ctx, "qa", string(c.ID)); !errors.Is(err, domain.ErrNotInCall) {
		t.Fatalf("member not in call = %v", err)
	}
	if fmt.Sprint(rec.log) != "[start k-1]" {
		t.Fatalf("egress started for refused requests: %v", rec.log)
	}
	from := relay(t, ctx, svc, repo, 0)
	if _, err := svc.StopRecording(ctx, "dev", string(c.ID)); err != nil {
		t.Fatal(err)
	}
	relay(t, ctx, svc, repo, from)
	if fmt.Sprint(rec.log) != "[start k-1 stop EG_k-1 open EG_k-1 discard EG_k-1]" {
		t.Fatalf("recorder = %v", rec.log)
	}
	if fmt.Sprint(*out) != `[dev dev "Huddle 2026-10-14 09-05.mp4" video of EG_k-1]` {
		t.Fatalf("posted = %v", *out)
	}

	// Not finished yet: the relay retries later, nothing is posted or discarded.
	pending := errors.New("still writing")
	rec.open, rec.log = pending, nil
	payload, _ := json.Marshal(domain.RecordingStopped{Call: c.ID, Conversation: "dev", By: "dev", Egress: "EG_2"})
	if err := svc.OnRecordingStopped(ctx, outbox.Message{Payload: payload}); !errors.Is(err, pending) {
		t.Fatalf("pending = %v", err)
	}
	if fmt.Sprint(rec.log) != "[stop EG_2 open EG_2]" || len(*out) != 1 {
		t.Fatalf("pending log = %v posted %v", rec.log, *out)
	}
	rec.open = app.ErrRecordingLost
	if err := svc.OnRecordingStopped(ctx, outbox.Message{Payload: payload}); err != nil {
		t.Fatalf("lost = %v", err)
	}
	if err := svc.OnRecordingStopped(ctx, outbox.Message{Payload: []byte("{bad")}); err != nil {
		t.Fatalf("poison = %v", err)
	}
}

func TestServerRecordingFallsBack(t *testing.T) {
	svc, _, rec, _, _, ctx := roomService(t)
	c, _ := svc.Join(ctx, "lead", "dev")
	rec.startErr = errors.New("egress unavailable")
	if _, err := svc.StartRecording(ctx, "lead", string(c.ID)); !errors.Is(err, rec.startErr) {
		t.Fatalf("egress down = %v", err)
	}
	// Without a recorder a room call is recorded in the browser, as a mesh call.
	_, ctx2, _ := setup()
	plain := app.NewService(app.Deps{Calls: memory.NewRepository(), Conversations: convs{"dev": {"lead"}}, Rooms: rooms{},
		NewID: func() string { return "k-9" }, Now: time.Now})
	c, _ = plain.Join(ctx2, "lead", "dev")
	got, err := plain.StartRecording(ctx2, "lead", string(c.ID))
	if err != nil || got.Recording.Egress != "" {
		t.Fatalf("browser recording = %+v %v", got.Recording, err)
	}
	payload, _ := json.Marshal(domain.RecordingStopped{Call: c.ID, Conversation: "dev", By: "lead", Egress: "EG_x"})
	if err := plain.OnRecordingStopped(ctx2, outbox.Message{Payload: payload}); err != nil {
		t.Fatalf("no recorder = %v", err)
	}
}
