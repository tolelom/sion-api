package handlers

import (
	"encoding/json"
	"net"
	"sion-backend/models"
	"sion-backend/services"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"
)

type wsTestServer struct {
	addr string
	cm   *services.ClientManager
	sim  *services.AGVSimulator
}

func newWSTestServer(t *testing.T) *wsTestServer {
	t.Helper()
	return newWSTestServerWith(t, services.NewClientManager())
}

func newWSTestServerWith(t *testing.T, cm *services.ClientManager) *wsTestServer {
	t.Helper()
	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) { cm.BroadcastJSON(msg) }, func() bool { return cm.Count() > 0 })

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use("/websocket", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/websocket/web", fiberws.New(NewWebHandler(cm, sim)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return &wsTestServer{addr: ln.Addr().String(), cm: cm, sim: sim}
}

func (s *wsTestServer) dial(t *testing.T, path string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial("ws://"+s.addr+path, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func readUntilType(t *testing.T, c *websocket.Conn, want string, deadline time.Duration) models.WebSocketMessage {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		c.SetReadDeadline(end)
		_, raw, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %s: %v", want, err)
		}
		var msg models.WebSocketMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("decode: %v (raw=%s)", err, raw)
		}
		if msg.Type == want {
			return msg
		}
	}
	t.Fatalf("no %s message within %v", want, deadline)
	return models.WebSocketMessage{}
}

func waitFor(t *testing.T, deadline time.Duration, cond func() bool, msg string) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", msg)
}

func sendJSON(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write: %v", err)
	}
}
