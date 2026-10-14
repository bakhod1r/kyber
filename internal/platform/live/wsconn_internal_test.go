package live

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// pair returns the server side of a WebSocket wrapped as wsConn and the client side.
func pair(t *testing.T) (*wsConn, *websocket.Conn) {
	t.Helper()
	server := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		server <- c
	}))
	t.Cleanup(srv.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return &wsConn{c: <-server}, client
}

func TestWSConnReadsAcrossMessages(t *testing.T) {
	ws, client := pair(t)
	_ = client.WriteMessage(websocket.BinaryMessage, []byte("ab"))
	_ = client.WriteMessage(websocket.BinaryMessage, []byte("cd"))
	buf := make([]byte, 2)
	var got []byte
	for len(got) < 4 {
		n, err := ws.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, buf[:n]...)
	}
	if string(got) != "abcd" {
		t.Fatalf("read %q", got)
	}
	_ = client.WriteMessage(websocket.TextMessage, []byte("{}"))
	if _, err := ws.Read(buf); !errors.Is(err, errTextFrame) {
		t.Fatalf("text frame: %v", err)
	}
}

func TestWSConnWriteAndClose(t *testing.T) {
	ws, client := pair(t)
	if n, err := ws.Write([]byte("x")); err != nil || n != 1 {
		t.Fatalf("write = %d %v", n, err)
	}
	if _, data, err := client.ReadMessage(); err != nil || string(data) != "x" {
		t.Fatalf("client read %q %v", data, err)
	}
	_ = ws.Close()
	if _, err := ws.Write([]byte("y")); err == nil {
		t.Fatal("write after close succeeded")
	}
	client.Close()
	if _, err := ws.Read(make([]byte, 1)); err == nil {
		t.Fatal("read after close succeeded")
	}
}
