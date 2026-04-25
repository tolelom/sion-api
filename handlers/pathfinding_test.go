package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newPathfindingApp() *fiber.App {
	app := fiber.New()
	app.Post("/api/pathfinding", HandlePathfinding)
	return app
}

func doPathfinding(t *testing.T, app *fiber.App, body any) (int, PathfindingResponse) {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("request 인코딩 실패: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/pathfinding", &buf)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("응답 읽기 실패: %v", err)
	}

	var out PathfindingResponse
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("응답 디코딩 실패: %v (body=%s)", err, string(raw))
		}
	}
	return resp.StatusCode, out
}

func TestHandlePathfinding_StraightLine(t *testing.T) {
	app := newPathfindingApp()

	body := map[string]any{
		"start":      map[string]float64{"x": 0, "y": 0},
		"goal":       map[string]float64{"x": 4, "y": 0},
		"map_width":  5,
		"map_height": 5,
	}

	status, resp := doPathfinding(t, app, body)
	if status != http.StatusOK {
		t.Fatalf("HTTP 200 기대, got %d", status)
	}
	if !resp.Success {
		t.Fatalf("success=true 기대, got %+v", resp)
	}
	if len(resp.Path) != 5 {
		t.Fatalf("5포인트 기대, got %d (%v)", len(resp.Path), resp.Path)
	}
	if resp.Path[0].X != 0 || resp.Path[0].Y != 0 {
		t.Fatalf("시작점 (0,0) 기대, got %+v", resp.Path[0])
	}
	if resp.Path[4].X != 4 || resp.Path[4].Y != 0 {
		t.Fatalf("끝점 (4,0) 기대, got %+v", resp.Path[4])
	}
}

func TestHandlePathfinding_StartEqualsGoal(t *testing.T) {
	app := newPathfindingApp()

	body := map[string]any{
		"start":      map[string]float64{"x": 2, "y": 2},
		"goal":       map[string]float64{"x": 2, "y": 2},
		"map_width":  5,
		"map_height": 5,
	}
	status, resp := doPathfinding(t, app, body)
	if status != http.StatusOK || !resp.Success {
		t.Fatalf("성공 기대, got status=%d resp=%+v", status, resp)
	}
	if len(resp.Path) != 1 {
		t.Fatalf("1포인트 기대, got %d", len(resp.Path))
	}
}

func TestHandlePathfinding_Unreachable(t *testing.T) {
	app := newPathfindingApp()

	body := map[string]any{
		"start":      map[string]float64{"x": 0, "y": 0},
		"goal":       map[string]float64{"x": 4, "y": 4},
		"map_width":  5,
		"map_height": 5,
		"obstacles": []map[string]int{
			{"x": 4, "y": 3},
			{"x": 3, "y": 4},
			{"x": 3, "y": 3},
		},
	}
	status, resp := doPathfinding(t, app, body)
	// 핸들러는 실패 시에도 200 + success=false로 응답한다
	if status != http.StatusOK {
		t.Fatalf("HTTP 200 기대, got %d", status)
	}
	if resp.Success {
		t.Fatalf("도달 불가능 → success=false 기대, got %+v", resp)
	}
	if resp.Message == "" {
		t.Fatal("실패 메시지 기대")
	}
	if len(resp.Path) != 0 {
		t.Fatalf("실패 시 path 비어있어야 함, got %v", resp.Path)
	}
}

func TestHandlePathfinding_BadRequest(t *testing.T) {
	app := newPathfindingApp()

	req := httptest.NewRequest(http.MethodPost, "/api/pathfinding",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("HTTP 400 기대, got %d", resp.StatusCode)
	}

	var out PathfindingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("응답 디코딩 실패: %v", err)
	}
	if out.Success {
		t.Fatal("success=false 기대")
	}
}

func TestHandlePathfinding_GoalOnObstacle(t *testing.T) {
	app := newPathfindingApp()

	body := map[string]any{
		"start":      map[string]float64{"x": 0, "y": 0},
		"goal":       map[string]float64{"x": 3, "y": 3},
		"map_width":  5,
		"map_height": 5,
		"obstacles":  []map[string]int{{"x": 3, "y": 3}},
	}
	status, resp := doPathfinding(t, app, body)
	if status != http.StatusOK {
		t.Fatalf("HTTP 200 기대, got %d", status)
	}
	if resp.Success {
		t.Fatalf("장애물 위 목표 → success=false 기대, got %+v", resp)
	}
}
