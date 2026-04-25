package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sion-backend/models"
	"sion-backend/services"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// setupLogsApp는 인메모리 DB + Fiber app + logs 라우트를 셋업하고
// 테스트 종료 시 db를 nil로 복원하는 cleanup 함수를 반환한다.
func setupLogsApp(t *testing.T) (*fiber.App, *gorm.DB, func()) {
	t.Helper()
	gdb, err := services.NewInMemoryDB()
	if err != nil {
		t.Fatalf("NewInMemoryDB 실패: %v", err)
	}
	services.SetTestDB(gdb)

	app := fiber.New()
	app.Get("/api/logs/recent", HandleGetRecentLogs)
	app.Get("/api/logs/range", HandleGetLogsByTimeRange)
	app.Get("/api/logs/event-type", HandleGetLogsByEventType)
	app.Get("/api/logs/stats", HandleGetLogStats)

	return app, gdb, func() { services.SetTestDB(nil) }
}

func seedLog(t *testing.T, gdb *gorm.DB, l models.AGVLog) {
	t.Helper()
	if err := gdb.Create(&l).Error; err != nil {
		t.Fatalf("seed 실패: %v", err)
	}
}

func doGet(t *testing.T, app *fiber.App, target string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test 실패: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("body read 실패: %v", err)
	}
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("응답 디코드 실패 (%d): %s", resp.StatusCode, raw)
		}
	}
	return resp.StatusCode, out
}

// =====================================================================
// HandleGetRecentLogs
// =====================================================================

func TestHandleGetRecentLogs_Empty(t *testing.T) {
	app, _, cleanup := setupLogsApp(t)
	defer cleanup()

	status, body := doGet(t, app, "/api/logs/recent?agv_id=sion-001")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 0 {
		t.Fatalf("expected count 0, got %v", body["count"])
	}
	if logs, ok := body["logs"].([]any); !ok || len(logs) != 0 {
		t.Fatalf("expected empty logs, got %v", body["logs"])
	}
}

func TestHandleGetRecentLogs_LimitAndOrder(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	now := time.Now()
	for i := 0; i < 5; i++ {
		seedLog(t, gdb, models.AGVLog{
			AGVID:     "sion-001",
			EventType: "position",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	status, body := doGet(t, app, "/api/logs/recent?agv_id=sion-001&limit=3")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 3 {
		t.Fatalf("expected count 3, got %v", body["count"])
	}
	logs := body["logs"].([]any)
	// DESC로 정렬되어야 하므로 첫 항목이 가장 최근
	first := logs[0].(map[string]any)
	last := logs[2].(map[string]any)
	if first["created_at"].(string) <= last["created_at"].(string) {
		t.Fatalf("expected DESC ordering, got first=%v last=%v", first["created_at"], last["created_at"])
	}
}

func TestHandleGetRecentLogs_InvalidLimitFallsBackTo100(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	// 1개만 시드 — invalid limit이어도 100으로 폴백되어 1개 반환되면 성공
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x"})

	status, body := doGet(t, app, "/api/logs/recent?agv_id=sion-001&limit=not-a-number")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 1 {
		t.Fatalf("expected count 1, got %v", body["count"])
	}
}

func TestHandleGetRecentLogs_FilterByAGVID(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x"})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-002", EventType: "x"})

	status, body := doGet(t, app, "/api/logs/recent?agv_id=sion-002")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 1 {
		t.Fatalf("expected count 1 for sion-002, got %v", body["count"])
	}
}

// =====================================================================
// HandleGetLogsByTimeRange
// =====================================================================

func TestHandleGetLogsByTimeRange_FilterByRange(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	base := time.Now().UTC().Truncate(time.Second)
	// 범위 안 2개, 범위 밖 1개
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x", CreatedAt: base.Add(-2 * time.Hour)})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x", CreatedAt: base.Add(-1 * time.Hour)})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x", CreatedAt: base.Add(-10 * time.Hour)})

	start := base.Add(-3 * time.Hour).Format(time.RFC3339)
	end := base.Format(time.RFC3339)
	status, body := doGet(t, app, "/api/logs/range?agv_id=sion-001&start="+start+"&end="+end)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 2 {
		t.Fatalf("expected count 2 in range, got %v", body["count"])
	}
}

func TestHandleGetLogsByTimeRange_BadStartFormat(t *testing.T) {
	app, _, cleanup := setupLogsApp(t)
	defer cleanup()

	status, body := doGet(t, app, "/api/logs/range?agv_id=sion-001&start=not-a-date")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if body["error"] == nil {
		t.Fatalf("expected error message, got %v", body)
	}
}

// =====================================================================
// HandleGetLogsByEventType
// =====================================================================

func TestHandleGetLogsByEventType_RequiresEventType(t *testing.T) {
	app, _, cleanup := setupLogsApp(t)
	defer cleanup()

	status, body := doGet(t, app, "/api/logs/event-type?agv_id=sion-001")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if body["error"] == nil {
		t.Fatalf("expected error, got %v", body)
	}
}

func TestHandleGetLogsByEventType_FiltersByType(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "position"})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "position"})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "command"})

	status, body := doGet(t, app, "/api/logs/event-type?agv_id=sion-001&event_type=position")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if body["count"].(float64) != 2 {
		t.Fatalf("expected count 2 for event_type=position, got %v", body["count"])
	}
	if body["event_type"].(string) != "position" {
		t.Fatalf("expected event_type=position in response, got %v", body["event_type"])
	}
}

// =====================================================================
// HandleGetLogStats
// =====================================================================

func TestHandleGetLogStats_AggregatesByEventType(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	now := time.Now()
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "position", CreatedAt: now})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "position", CreatedAt: now})
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "command", CreatedAt: now})
	// 24시간 윈도우 밖의 오래된 로그는 집계되지 않아야 함
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "command", CreatedAt: now.Add(-48 * time.Hour)})

	status, body := doGet(t, app, "/api/logs/stats?agv_id=sion-001&hours=24")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	stats := body["stats"].(map[string]any)
	if stats["total_logs"].(float64) != 3 {
		t.Fatalf("expected total_logs=3 within 24h, got %v", stats["total_logs"])
	}
	counts := stats["event_counts"].(map[string]any)
	if counts["position"].(float64) != 2 || counts["command"].(float64) != 1 {
		t.Fatalf("unexpected event_counts: %v", counts)
	}
}

func TestHandleGetLogStats_InvalidHoursFallsBackTo24(t *testing.T) {
	app, gdb, cleanup := setupLogsApp(t)
	defer cleanup()

	now := time.Now()
	seedLog(t, gdb, models.AGVLog{AGVID: "sion-001", EventType: "x", CreatedAt: now})

	status, body := doGet(t, app, "/api/logs/stats?agv_id=sion-001&hours=oops")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	stats := body["stats"].(map[string]any)
	if stats["time_range"].(string) != "Last 24 hours" {
		t.Fatalf("expected fallback to 24h, got %v", stats["time_range"])
	}
}
