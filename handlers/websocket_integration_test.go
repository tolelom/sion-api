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

// wsTestServer는 ephemeral 포트에 Fiber 앱을 띄우고 AGV/Web WS 라우트를 등록한다.
// LLM은 nil — 핸들러의 chat 분기는 nil-check가 있어 통합 테스트에 영향 없음.
type wsTestServer struct {
	addr   string
	cm     *services.ClientManager
	broker *services.Broker
	app    *fiber.App
}

func newWSTestServer(t *testing.T) *wsTestServer {
	t.Helper()

	cm := services.NewClientManager()
	br := services.NewBroker(cm)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use("/websocket", func(c *fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/websocket/agv", fiberws.New(NewAGVHandler(cm, br)))
	app.Get("/websocket/web", fiberws.New(NewWebHandler(cm, br, nil)))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 실패: %v", err)
	}
	go func() {
		// Listener는 Shutdown 호출 시 ErrServerClosed로 깔끔하게 종료된다.
		_ = app.Listener(ln)
	}()

	srv := &wsTestServer{
		addr:   ln.Addr().String(),
		cm:     cm,
		broker: br,
		app:    app,
	}
	t.Cleanup(func() {
		_ = app.Shutdown()
	})
	return srv
}

func (s *wsTestServer) dial(t *testing.T, path string) *websocket.Conn {
	t.Helper()
	url := "ws://" + s.addr + path
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s 실패: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readJSON은 deadline 안에 한 메시지를 받아 디코드한다.
func readJSON(t *testing.T, c *websocket.Conn, dst any, deadline time.Duration) {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(deadline)); err != nil {
		t.Fatalf("SetReadDeadline 실패: %v", err)
	}
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 실패: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("응답 디코드 실패: %v (raw=%s)", err, raw)
	}
}

// readUntilType은 특정 type의 메시지가 올 때까지 deadline 동안 읽는다.
// system_info 같은 welcome 메시지를 건너뛰는 데 사용.
func readUntilType(t *testing.T, c *websocket.Conn, want string, deadline time.Duration) models.WebSocketMessage {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if err := c.SetReadDeadline(end); err != nil {
			t.Fatalf("SetReadDeadline 실패: %v", err)
		}
		_, raw, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage 실패: %v", err)
		}
		var msg models.WebSocketMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("디코드 실패: %v (raw=%s)", err, raw)
		}
		if msg.Type == want {
			return msg
		}
	}
	t.Fatalf("type=%s 메시지를 %v 안에 받지 못함", want, deadline)
	return models.WebSocketMessage{}
}

// waitFor는 cond가 true가 될 때까지 짧게 폴링한다. WS 핸들러의 register/disconnect는 비동기라 즉시 검사하면 race.
func waitFor(t *testing.T, deadline time.Duration, cond func() bool, msg string) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitFor timeout: %s", msg)
}

// =====================================================================
// 1) AGV 연결 시 broker.IsAGVConnected가 true로 전이
// =====================================================================
func TestWS_AGVConnect_SetsAGVConnected(t *testing.T) {
	srv := newWSTestServer(t)

	if srv.broker.IsAGVConnected() {
		t.Fatalf("초기에 AGV connected가 true이면 안 됨")
	}

	agv := srv.dial(t, "/websocket/agv")
	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV가 연결되어도 IsAGVConnected가 true가 되지 않음")
	_ = agv.Close()
	waitFor(t, 1*time.Second, func() bool { return !srv.broker.IsAGVConnected() }, "AGV 연결 종료 후에도 IsAGVConnected가 false가 되지 않음")
}

// =====================================================================
// 2) AGV → Web 브로드캐스트
// =====================================================================
func TestWS_AGVStatusBroadcastsToWeb(t *testing.T) {
	srv := newWSTestServer(t)

	web := srv.dial(t, "/websocket/web")
	// 첫 메시지는 system_info welcome — 소비
	readUntilType(t, web, models.MessageTypeSystemInfo, 1*time.Second)

	agv := srv.dial(t, "/websocket/agv")
	// AGV가 등록 완료되어 broker가 connected = true 되면 web에는 agv_connected 메시지가 가지만
	// 우린 곧장 status를 보낼 거라 그 알림은 readUntilType이 건너뛴다.
	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV connected wait")

	statusMsg := models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: map[string]any{
			"position": map[string]any{"x": 1.0, "y": 2.0, "angle": 0.0},
			"battery":  77,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	raw, _ := json.Marshal(statusMsg)
	if err := agv.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("AGV WriteMessage 실패: %v", err)
	}

	got := readUntilType(t, web, models.MessageTypeStatus, 1*time.Second)
	data, ok := got.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data가 map이 아님: %T", got.Data)
	}
	if data["battery"].(float64) != 77 {
		t.Fatalf("expected battery=77, got %v", data["battery"])
	}

	// broker가 마지막 status를 캐시했는지도 검증
	cached := srv.broker.GetAGVStatus()
	if cached == nil {
		t.Fatalf("broker.GetAGVStatus가 nil")
	}
	if cached.Battery != 77 {
		t.Fatalf("expected cached battery=77, got %d", cached.Battery)
	}
}

// =====================================================================
// 3) Web → AGV 명령 라우팅
// =====================================================================
func TestWS_WebCommandRoutesToAGV(t *testing.T) {
	srv := newWSTestServer(t)

	agv := srv.dial(t, "/websocket/agv")
	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV connected wait")

	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeSystemInfo, 1*time.Second)

	cmd := models.WebSocketMessage{
		Type: models.MessageTypeCommand,
		Data: map[string]any{"action": "go", "x": 5, "y": 6},
	}
	raw, _ := json.Marshal(cmd)
	if err := web.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("Web WriteMessage 실패: %v", err)
	}

	got := readUntilType(t, agv, models.MessageTypeCommand, 1*time.Second)
	data := got.Data.(map[string]any)
	if data["action"].(string) != "go" {
		t.Fatalf("expected action=go, got %v", data["action"])
	}
}

// =====================================================================
// 4) Web 잘못된 JSON → MessageTypeError 응답
// =====================================================================
func TestWS_WebInvalidJSON_ReceivesError(t *testing.T) {
	srv := newWSTestServer(t)

	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeSystemInfo, 1*time.Second)

	if err := web.WriteMessage(websocket.TextMessage, []byte("not-json")); err != nil {
		t.Fatalf("WriteMessage 실패: %v", err)
	}

	got := readUntilType(t, web, models.MessageTypeError, 1*time.Second)
	data := got.Data.(map[string]any)
	if data["message"] == nil {
		t.Fatalf("error.message가 nil: %v", got.Data)
	}
}

// =====================================================================
// 5) AGV 연결 끊김 → Web에 agv_disconnected 알림
// =====================================================================
func TestWS_AGVDisconnect_NotifiesWeb(t *testing.T) {
	srv := newWSTestServer(t)

	agv := srv.dial(t, "/websocket/agv")
	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV connected wait")

	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeSystemInfo, 1*time.Second)

	if err := agv.Close(); err != nil {
		t.Fatalf("AGV close 실패: %v", err)
	}

	got := readUntilType(t, web, models.MessageTypeAGVDisconnected, 2*time.Second)
	data := got.Data.(map[string]any)
	if data["connected"].(bool) {
		t.Fatalf("expected connected=false, got %v", data["connected"])
	}
	if srv.broker.IsAGVConnected() {
		t.Fatalf("broker.IsAGVConnected가 여전히 true")
	}
}
