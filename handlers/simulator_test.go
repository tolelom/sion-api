package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sion-backend/models"
	"sion-backend/services"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newSimulatorApp(sim *services.AGVSimulator) *fiber.App {
	app := fiber.New()
	app.Post("/api/simulator/start", NewSimulatorStartHandler(sim))
	app.Post("/api/simulator/stop", NewSimulatorStopHandler(sim))
	app.Get("/api/simulator/status", NewSimulatorStatusHandler(sim))
	return app
}

func doSimReq(t *testing.T, app *fiber.App, method, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("응답 읽기 실패: %v", err)
	}

	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("응답 디코딩 실패: %v (body=%s)", err, string(raw))
		}
	}
	return resp.StatusCode, out
}

func TestSimulatorHandlers_LifecycleHappyPath(t *testing.T) {
	sim := services.NewAGVSimulator(func(_ models.WebSocketMessage) {})
	app := newSimulatorApp(sim)

	// 초기 상태: 정지
	status, body := doSimReq(t, app, http.MethodGet, "/api/simulator/status")
	if status != http.StatusOK {
		t.Fatalf("status HTTP 200 기대, got %d", status)
	}
	if running, ok := body["running"].(bool); !ok || running {
		t.Fatalf("초기 running=false 기대, body=%+v", body)
	}

	// start
	status, body = doSimReq(t, app, http.MethodPost, "/api/simulator/start")
	if status != http.StatusOK {
		t.Fatalf("start HTTP 200 기대, got %d", status)
	}
	if success, _ := body["success"].(bool); !success {
		t.Fatalf("start success=true 기대, body=%+v", body)
	}

	// stop으로 종료 보장 (deferred)
	defer func() {
		_, _ = doSimReq(t, app, http.MethodPost, "/api/simulator/stop")
	}()

	// 실행 중 상태 확인
	status, body = doSimReq(t, app, http.MethodGet, "/api/simulator/status")
	if running, _ := body["running"].(bool); !running {
		t.Fatalf("start 후 running=true 기대, body=%+v", body)
	}
	if _, ok := body["enemies"]; !ok {
		t.Fatal("status 응답에 enemies 필드 기대")
	}
	mapSize, ok := body["map_size"].(map[string]any)
	if !ok {
		t.Fatalf("map_size 객체 기대, body=%+v", body)
	}
	if _, ok := mapSize["width"]; !ok {
		t.Fatal("map_size.width 기대")
	}
}

func TestSimulatorHandlers_StartTwiceFails(t *testing.T) {
	sim := services.NewAGVSimulator(func(_ models.WebSocketMessage) {})
	app := newSimulatorApp(sim)

	if status, _ := doSimReq(t, app, http.MethodPost, "/api/simulator/start"); status != http.StatusOK {
		t.Fatalf("첫 start 200 기대, got %d", status)
	}
	defer func() {
		_, _ = doSimReq(t, app, http.MethodPost, "/api/simulator/stop")
	}()

	status, body := doSimReq(t, app, http.MethodPost, "/api/simulator/start")
	if status != http.StatusBadRequest {
		t.Fatalf("이중 start → 400 기대, got %d", status)
	}
	if success, _ := body["success"].(bool); success {
		t.Fatalf("success=false 기대, body=%+v", body)
	}
}

func TestSimulatorHandlers_StopWhenNotRunningFails(t *testing.T) {
	sim := services.NewAGVSimulator(func(_ models.WebSocketMessage) {})
	app := newSimulatorApp(sim)

	status, body := doSimReq(t, app, http.MethodPost, "/api/simulator/stop")
	if status != http.StatusBadRequest {
		t.Fatalf("미실행 시 stop → 400 기대, got %d", status)
	}
	if success, _ := body["success"].(bool); success {
		t.Fatalf("success=false 기대, body=%+v", body)
	}
}
