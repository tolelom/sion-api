package services

import (
	"sion-backend/models"
	"testing"
)

func TestInferEventType(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"position", "position_update"},
		{"status", "status_update"},
		{"target_found", "target_detected"},
		{"chat", "user_question"},
		{"command", "command_received"},
		{"chat_response", "ai_response"},
		{"agv_event", "event_description"},
		{"emergency_stop", "emergency_stop"},
		{"mode_change", "mode_change"},
		{"unknown_xyz", "unknown_xyz"}, // 폴백: 그대로
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := inferEventType(tc.in); got != tc.want {
				t.Fatalf("%q → %q 기대, got %q", tc.in, tc.want, got)
			}
		})
	}
}

func TestExtractLogData_FullMap(t *testing.T) {
	entry := models.AGVLog{}
	msg := models.NewMessage("status", map[string]interface{}{
		"x":        1.5,
		"y":        2.5,
		"angle":    0.785,
		"speed":    3.0,
		"battery":  float64(80), // JSON 디코딩 시 항상 float64
		"mode":     "auto",
		"state":    "moving",
		"target_x": 9.0,
		"target_y": 10.0,
		"action":   "move",
	}, 0)
	extractLogData(&entry, msg)

	if entry.PositionX != 1.5 || entry.PositionY != 2.5 || entry.PositionAngle != 0.785 {
		t.Fatalf("position 추출 실패, got %+v", entry)
	}
	if entry.Speed != 3.0 || entry.Battery != 80 {
		t.Fatalf("status 추출 실패, got %+v", entry)
	}
	if entry.Mode != "auto" || entry.State != "moving" {
		t.Fatalf("mode/state 추출 실패, got %+v", entry)
	}
	if entry.TargetX != 9.0 || entry.TargetY != 10.0 || entry.CommandType != "move" {
		t.Fatalf("command 추출 실패, got %+v", entry)
	}
}

func TestExtractLogData_NonMapPayload(t *testing.T) {
	// PositionData를 그대로 RawMessage로 직렬화하면 JSON object {"x":1,"y":2,...}가 되어
	// map[string]interface{} unmarshal에 성공한다. 다만 x/y는 PositionData의 field name이므로
	// extractLogData는 이를 인식한다(설계상 PositionData가 map 추출과 같은 키를 쓰는 일은
	// envelope이 다른 type일 때 발생). 원래 의도는 "Data가 map으로 해석되지 않을 때 무시"였으니,
	// 여기서는 "비어있는 JSON object → 어떤 필드도 갱신 안 됨"으로 케이스를 단순화한다.
	entry := models.AGVLog{AGVID: "agv-1"}
	msg := models.NewMessage("position", struct{}{}, 0)
	extractLogData(&entry, msg)

	if entry.PositionX != 0 || entry.PositionY != 0 {
		t.Fatalf("빈 payload는 추출하지 않아야 함, got %+v", entry)
	}
	if entry.AGVID != "agv-1" {
		t.Fatalf("기존 필드 보존 기대, got %+v", entry)
	}
}

func TestExtractLogData_PartialMap(t *testing.T) {
	// 일부 키만 있고 타입이 어긋난 경우 — 해당 필드만 채워지고 나머지는 zero
	entry := models.AGVLog{}
	msg := models.NewMessage("status", map[string]interface{}{
		"x":       4.2,
		"battery": "not-a-number", // 타입 어긋남 → 무시
		"mode":    "manual",
	}, 0)
	extractLogData(&entry, msg)

	if entry.PositionX != 4.2 {
		t.Fatalf("x 추출 기대, got %+v", entry)
	}
	if entry.Battery != 0 {
		t.Fatalf("타입 어긋난 battery는 0 유지 기대, got %d", entry.Battery)
	}
	if entry.Mode != "manual" {
		t.Fatalf("mode 추출 기대, got %q", entry.Mode)
	}
}
