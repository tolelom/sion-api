package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sion-backend/models"
	"sion-backend/services"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
)

const testOrigin = "http://allowed.example"

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	cm := services.NewClientManager()
	sim := services.NewAGVSimulator(func(msg models.WebSocketMessage) { cm.BroadcastJSON(msg) }, func() bool { return cm.Count() > 0 })
	return newApp(cm, sim, testOrigin+", http://second.example")
}

func listen(t *testing.T, app *fiber.App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return ln.Addr().String()
}

func TestOriginList(t *testing.T) {
	got := originList(" http://a.example , ,http://b.example,")
	want := []string{"http://a.example", "http://b.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("originList = %#v, want %#v", got, want)
	}
	if got := originList(""); got != nil {
		t.Fatalf("originList(\"\") = %#v, want nil", got)
	}
}

func TestApp_Root(t *testing.T) {
	resp, err := newTestApp(t).Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "Sion WebSocket 서버가 실행 중입니다." {
		t.Fatalf("GET / = %d %q", resp.StatusCode, body)
	}
}

func TestApp_Health(t *testing.T) {
	resp, err := newTestApp(t).Test(httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 3 || body["status"] != "OK" || body["clients"] != float64(0) {
		t.Fatalf("health body = %v", body)
	}
	if _, err := time.Parse(time.RFC3339, body["time"].(string)); err != nil {
		t.Fatalf("time is not RFC3339: %v", body["time"])
	}
}

func TestApp_RejectsBodiesOver64KB(t *testing.T) {
	base := listen(t, newTestApp(t))
	resp, err := http.Post("http://"+base+"/api/pathfinding", "application/json", bytes.NewReader(bytes.Repeat([]byte("x"), 64*1024+1)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestApp_RecoversFromPanics(t *testing.T) {
	app := newTestApp(t)
	app.Get("/api/boom", func(c *fiber.Ctx) error { panic("boom") })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/boom", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestApp_CORS(t *testing.T) {
	cases := []struct {
		origin string
		want   string
	}{
		{testOrigin, testOrigin},
		{"http://second.example", "http://second.example"},
		{"http://evil.example", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("Origin", tc.origin)
		resp, err := newTestApp(t).Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Fatalf("origin %s: Access-Control-Allow-Origin = %q, want %q", tc.origin, got, tc.want)
		}
	}
}

func TestApp_WebsocketRequiresUpgrade(t *testing.T) {
	resp, err := newTestApp(t).Test(httptest.NewRequest(http.MethodGet, "/websocket/web", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want 426", resp.StatusCode)
	}
}

func TestApp_WebsocketChecksOrigin(t *testing.T) {
	url := "ws://" + listen(t, newTestApp(t)) + "/websocket/web"
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}

	conn, _, err := dialer.Dial(url, http.Header{"Origin": {testOrigin}})
	if err != nil {
		t.Fatalf("allowed origin should connect: %v", err)
	}
	conn.Close()

	if conn, resp, err := dialer.Dial(url, http.Header{"Origin": {"http://evil.example"}}); err == nil {
		conn.Close()
		t.Fatal("disallowed origin must not connect")
	} else if resp == nil || resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("disallowed origin: resp=%v err=%v, want 426", resp, err)
	}
}
