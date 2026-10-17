package handlers

import (
	"encoding/json"
	"sion-backend/models"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

// 명령 메시지의 경계 입력별 응답을 고정한다. 오류 문구는 클라이언트에 그대로 노출되는 계약이다.
// 성공한 명령은 응답이 없으므로, 뒤이어 보낸 probe의 오류가 첫 오류로 오는지로 확인한다.

const probeError = `지원하지 않는 메시지 타입: "probe"`

func dialReady(t *testing.T) (*wsTestServer, *websocket.Conn) {
	t.Helper()
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	return srv, web
}

func sendRaw(t *testing.T, c *websocket.Conn, raw string) {
	t.Helper()
	if err := c.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func firstErrorMessage(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	msg := readUntilType(t, c, models.MessageTypeError, time.Second)
	data, ok := msg.Data.(map[string]any)
	if !ok {
		t.Fatalf("error data is not an object: %#v", msg.Data)
	}
	text, _ := data["message"].(string)
	return text
}

func TestWSCommandContract_ErrorMessages(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"mode_change with null data", `{"type":"mode_change","data":null}`, `알 수 없는 모드: ""`},
		{"mode_change without data key", `{"type":"mode_change"}`, `알 수 없는 모드: ""`},
		{"mode_change with unknown mode", `{"type":"mode_change","data":{"mode":"turbo"}}`, `알 수 없는 모드: "turbo"`},
		{"mode_change with string data", `{"type":"mode_change","data":"manual"}`, "잘못된 명령 형식"},
		{"mode_change with wrong field type", `{"type":"mode_change","data":{"mode":5}}`, "잘못된 명령 형식"},
		{"command with null data", `{"type":"command","data":null}`, "경로가 비어 있습니다"},
		{"command without data key", `{"type":"command"}`, "경로가 비어 있습니다"},
		{"command with empty path", `{"type":"command","data":{"path":[]}}`, "경로가 비어 있습니다"},
		{"command with path off the map", `{"type":"command","data":{"path":[{"x":21,"y":1}]}}`, "경로가 맵을 벗어납니다"},
		{"command with negative point", `{"type":"command","data":{"path":[{"x":-0.5,"y":1}]}}`, "경로가 맵을 벗어납니다"},
		{"command with array data", `{"type":"command","data":[1,2]}`, "잘못된 명령 형식"},
		{"unknown type", `{"type":"chat","data":{"message":"hi"}}`, `지원하지 않는 메시지 타입: "chat"`},
		{"missing type", `{"data":{}}`, `지원하지 않는 메시지 타입: ""`},
		{"not json", `not-json`, "잘못된 메시지 형식"},
		{"data is invalid json", `{"type":"command","data":{]}`, "잘못된 메시지 형식"},
		{"number out of float64 range in data", `{"type":"emergency_stop","data":{"x":1e400}}`, "잘못된 메시지 형식"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, web := dialReady(t)
			sendRaw(t, web, tc.raw)
			if got := firstErrorMessage(t, web); got != tc.want {
				t.Fatalf("error message = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWSCommandContract_PathAtLimit(t *testing.T) {
	path := func(n int) string {
		points := make([]map[string]float64, n)
		for i := range points {
			points[i] = map[string]float64{"x": 1, "y": 1}
		}
		raw, _ := json.Marshal(map[string]any{"type": "command", "data": map[string]any{"path": points}})
		return string(raw)
	}

	t.Run("401 points are rejected", func(t *testing.T) {
		_, web := dialReady(t)
		sendRaw(t, web, path(401))
		if got := firstErrorMessage(t, web); got != "경로는 최대 400개 지점까지입니다" {
			t.Fatalf("error message = %q", got)
		}
	})
	t.Run("400 points are accepted and switch to manual", func(t *testing.T) {
		srv, web := dialReady(t)
		sendRaw(t, web, path(400))
		sendRaw(t, web, `{"type":"probe"}`)
		if got := firstErrorMessage(t, web); got != probeError {
			t.Fatalf("400 points should be accepted, got error %q", got)
		}
		status, _, _, _ := srv.sim.Snapshot()
		if status.Mode != models.ModeManual {
			t.Fatalf("mode = %q, want manual", status.Mode)
		}
	})
}

func TestWSCommandContract_AcceptedWithoutReply(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		check func(models.AGVStatus) bool
	}{
		{"emergency_stop with null data", `{"type":"emergency_stop","data":null}`,
			func(s models.AGVStatus) bool { return s.State == models.StateEmergency }},
		{"emergency_stop without data key", `{"type":"emergency_stop"}`,
			func(s models.AGVStatus) bool { return s.State == models.StateEmergency }},
		{"emergency_stop ignores data shape", `{"type":"emergency_stop","data":"anything"}`,
			func(s models.AGVStatus) bool { return s.State == models.StateEmergency }},
		{"mode_change to manual", `{"type":"mode_change","data":{"mode":"manual"}}`,
			func(s models.AGVStatus) bool { return s.Mode == models.ModeManual }},
		{"mode_change ignores extra fields", `{"type":"mode_change","data":{"mode":"manual","extra":1}}`,
			func(s models.AGVStatus) bool { return s.Mode == models.ModeManual }},
		{"command path on the map edge", `{"type":"command","data":{"path":[{"x":20,"y":0}]}}`,
			func(s models.AGVStatus) bool { return s.Mode == models.ModeManual }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, web := dialReady(t)
			sendRaw(t, web, tc.raw)
			sendRaw(t, web, `{"type":"probe"}`)
			if got := firstErrorMessage(t, web); got != probeError {
				t.Fatalf("command should be accepted silently, got error %q", got)
			}
			status, _, _, _ := srv.sim.Snapshot()
			if !tc.check(status) {
				t.Fatalf("unexpected simulator state: mode=%q state=%q", status.Mode, status.State)
			}
		})
	}
}

func TestWSCommandContract_ResetAfterEmergency(t *testing.T) {
	srv, web := dialReady(t)
	sendRaw(t, web, `{"type":"emergency_stop"}`)
	sendRaw(t, web, `{"type":"command","data":{"path":[{"x":3,"y":3}]}}`)
	if got := firstErrorMessage(t, web); got != "정지 상태입니다. 초기화 후 다시 시도하세요" {
		t.Fatalf("path during emergency: error = %q", got)
	}
	sendRaw(t, web, `{"type":"command","data":{"action":"reset"}}`)
	sendRaw(t, web, `{"type":"probe"}`)
	if got := firstErrorMessage(t, web); got != probeError {
		t.Fatalf("reset should be accepted silently, got error %q", got)
	}
	status, _, _, _ := srv.sim.Snapshot()
	if status.State == models.StateEmergency || status.Mode != models.ModeAuto || status.Battery != 100 {
		t.Fatalf("reset did not restore initial state: mode=%q state=%q battery=%d", status.Mode, status.State, status.Battery)
	}
}
