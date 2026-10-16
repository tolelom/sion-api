package handlers

import (
	"bytes"
	"encoding/json"
	"sion-backend/models"
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

func TestWS_OversizedMessageDropsClient(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	waitFor(t, time.Second, func() bool { return srv.cm.Count() == 1 }, "client not registered")
	_ = web.WriteMessage(websocket.TextMessage, bytes.Repeat([]byte("x"), 1<<20))
	waitFor(t, 2*time.Second, func() bool { return srv.cm.Count() == 0 }, "a 1MB message must close the connection")
}

func TestWS_CommandFloodIsThrottled(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	for i := 0; i < 50; i++ {
		sendJSON(t, web, map[string]any{"type": "mode_change", "data": map[string]any{"mode": "manual"}})
	}
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		web.SetReadDeadline(end)
		_, raw, err := web.ReadMessage()
		if err != nil {
			break
		}
		var msg models.WebSocketMessage
		json.Unmarshal(raw, &msg)
		if msg.Type == models.MessageTypeError && strings.Contains(raw2s(msg.Data), "너무 많") {
			return
		}
	}
	t.Fatal("50 commands in a burst must be throttled")
}

func raw2s(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
