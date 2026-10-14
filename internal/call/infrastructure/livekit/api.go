package livekit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/call/application"
)

// APIBase is where Kyber calls LiveKit's server API: APIURL, or the browsers' URL over http(s).
func (l *LiveKit) APIBase() string {
	if l.c.APIURL != "" {
		return l.c.APIURL
	}
	return l.Origins()[1]
}

// Records reports whether LiveKit Egress records room calls (a shared recordings directory is set).
func (l *LiveKit) Records() bool { return l.c.RecordingsDir != "" }

type twirpError struct{ Code, Msg string }

func (e *twirpError) Error() string { return "livekit: " + e.Code + ": " + e.Msg }

// call invokes one Twirp method with a token carrying only grant.
func (l *LiveKit) call(ctx context.Context, method string, grant map[string]any, in, out any) error {
	body, _ := json.Marshal(in)                                                                                             // maps and strings always marshal
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, l.APIBase()+"/twirp/livekit."+method, bytes.NewReader(body)) // URL checked in New
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.sign(claimsWith(l.c.Key, "kyber", time.Now(), time.Minute, grant)))
	res, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20)) // a cut-off body fails to decode below
	if res.StatusCode != http.StatusOK {
		te := &twirpError{Code: strconv.Itoa(res.StatusCode)}
		_ = json.Unmarshal(raw, te)
		return te
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func isCode(err error, codes ...string) bool {
	var te *twirpError
	if !errors.As(err, &te) {
		return false
	}
	for _, c := range codes {
		if te.Code == c {
			return true
		}
	}
	return false
}

// Evict disconnects user from room; someone not there is no error.
func (l *LiveKit) Evict(ctx context.Context, room, user string) error {
	err := l.call(ctx, "RoomService/RemoveParticipant", map[string]any{"room": room, "roomAdmin": true},
		map[string]string{"room": room, "identity": user}, nil)
	if isCode(err, "not_found") {
		return nil
	}
	return err
}

// Start records room as one MP4 (grid of everyone) into the egress directory.
func (l *LiveKit) Start(ctx context.Context, room string) (string, error) {
	name := fmt.Sprintf("%s-%d.mp4", room, time.Now().UnixNano())
	var out struct {
		EgressID string `json:"egress_id"`
	}
	err := l.call(ctx, "Egress/StartRoomCompositeEgress", map[string]any{"roomRecord": true}, map[string]any{
		"room_name": room, "layout": "grid",
		"file_outputs": []map[string]string{{"file_type": "MP4", "filepath": path.Join(l.c.EgressDir, name)}},
	}, &out)
	return out.EgressID, err
}

// Stop ends a recording; one that already ended is no error.
func (l *LiveKit) Stop(ctx context.Context, egress string) error {
	err := l.call(ctx, "Egress/StopEgress", map[string]any{"roomRecord": true}, map[string]string{"egress_id": egress}, nil)
	if isCode(err, "failed_precondition", "not_found") {
		return nil
	}
	return err
}

// status is a recording's state and the file it wrote (inside the egress directory).
func (l *LiveKit) status(ctx context.Context, egress string) (status, file string, err error) {
	var out struct {
		Items []struct {
			Status      string `json:"status"`
			FileResults []struct {
				Filename string `json:"filename"`
			} `json:"file_results"`
		} `json:"items"`
	}
	if err := l.call(ctx, "Egress/ListEgress", map[string]any{"roomRecord": true}, map[string]string{"egress_id": egress}, &out); err != nil {
		return "", "", err
	}
	if len(out.Items) == 0 {
		return "", "", application.ErrRecordingLost
	}
	it := out.Items[0]
	if len(it.FileResults) > 0 {
		file = it.FileResults[0].Filename
	}
	return it.Status, file, nil
}

// local maps the egress-side file onto the recordings directory; only plain names inside the egress directory.
func (l *LiveKit) local(file string) (string, error) {
	if path.Dir(path.Clean(file)) != path.Clean(l.c.EgressDir) || path.Clean(file) != file {
		return "", application.ErrRecordingLost
	}
	return filepath.Join(l.c.RecordingsDir, path.Base(file)), nil
}

var errPending = errors.New("livekit: the recording is still being written")

// Open waits (up to Wait) for the recording to finish and opens its file.
func (l *LiveKit) Open(ctx context.Context, egress string) (io.ReadCloser, error) {
	deadline := time.Now().Add(l.c.Wait)
	for {
		status, file, err := l.status(ctx, egress)
		if err != nil {
			return nil, err
		}
		switch status {
		case "EGRESS_COMPLETE", "EGRESS_LIMIT_REACHED":
			name, err := l.local(file)
			if err != nil {
				return nil, err
			}
			f, err := os.Open(name)
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s", application.ErrRecordingLost, path.Base(file))
			}
			return f, err
		case "EGRESS_FAILED", "EGRESS_ABORTED":
			return nil, application.ErrRecordingLost
		}
		if time.Now().After(deadline) {
			return nil, errPending
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(l.c.Poll):
		}
	}
}

// Discard removes a posted recording's file and the manifest Egress writes next to it (<egress id>.json); files
// already gone are no error.
func (l *LiveKit) Discard(ctx context.Context, egress string) error {
	if egress == "" || path.Base(egress) != egress {
		return application.ErrRecordingLost
	}
	_, file, err := l.status(ctx, egress)
	if err != nil {
		return err
	}
	name, err := l.local(file)
	if err != nil {
		return err
	}
	for _, f := range []string{name, filepath.Join(l.c.RecordingsDir, egress+".json")} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
