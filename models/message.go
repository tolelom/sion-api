package models

import (
	"encoding/json"
	"log"
	"time"
)

// AGV -> Server -> Web
const (
	MessageTypePosition    = "position"
	MessageTypeStatus      = "status"
	MessageTypeLog         = "log"
	MessageTypeTargetFound = "target_found"
	MessageTypePathUpdate  = "path_update"
)

// Web -> Server -> AGV
const (
	MessageTypeCommand       = "command"
	MessageTypeModeChange    = "mode_change"
	MessageTypeEmergencyStop = "emergency_stop"
)

// Chat
const (
	MessageTypeChat         = "chat"
	MessageTypeChatResponse = "chat_response"
	MessageTypeAGVEvent     = "agv_event"
)

// LLM / System / Connection / Error
const (
	MessageTypeLLMExplanation  = "llm_explanation"
	MessageTypeTTS             = "tts"
	MessageTypeMapUpdate       = "map_update"
	MessageTypeSystemInfo      = "system_info"
	MessageTypeAGVConnected    = "agv_connected"
	MessageTypeAGVDisconnected = "agv_disconnected"
	MessageTypeError           = "error"
)

// WebSocketMessage의 Data는 json.RawMessage로 유지한다.
// interface{} 시절에는 호출 측이 map/struct를 그대로 넣고 소비 측이 동적 캐스트 또는
// re-marshal/unmarshal로 풀어야 해 직렬화가 1~3회 반복됐다. RawMessage는 생성 시점에
// 한 번 marshal해 두고 소비 시점에 strongly-typed unmarshal 1회만 한다.
type WebSocketMessage struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// NewMessage는 data를 즉시 marshal해 WebSocketMessage로 감싼다.
// data가 nil이면 Data는 nil이 된다.
// timestamp가 0이면 호출 시각으로 채운다(생성 호출 대부분이 time.Now().UnixMilli()을 그대로 넘기던 패턴 단순화).
// marshal 실패는 plain struct/map에서는 사실상 발생하지 않으므로 로그만 남기고 빈 Data로 반환한다.
func NewMessage(msgType string, data any, timestamp int64) WebSocketMessage {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			log.Printf("[WARN] NewMessage(%s) marshal 실패: %v", msgType, err)
		} else {
			raw = b
		}
	}
	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}
	return WebSocketMessage{
		Type:      msgType,
		Data:      raw,
		Timestamp: timestamp,
	}
}

type PositionData struct {
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Angle     float64   `json:"angle"`
	Timestamp time.Time `json:"timestamp"`
}

type MoveCommand struct {
	TargetX float64 `json:"target_x"`
	TargetY float64 `json:"target_y"`
	Mode    string  `json:"mode"`
}

type ModeChangeCommand struct {
	Mode string `json:"mode"`
}

type EmergencyStopCommand struct {
	Reason string `json:"reason"`
}

type PathData struct {
	Points    []PositionData `json:"points"`
	Length    float64        `json:"length"`
	Algorithm string        `json:"algorithm"`
	CreatedAt time.Time     `json:"created_at"`
}

type LLMExplanation struct {
	Text      string    `json:"text"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type TTSData struct {
	Text     string `json:"text"`
	AudioURL string `json:"audio_url"`
	Voice    string `json:"voice"`
}

type SystemInfo struct {
	ConnectedClients int       `json:"connected_clients"`
	AGVConnected     bool      `json:"agv_connected"`
	ServerTime       time.Time `json:"server_time"`
	Uptime           int64     `json:"uptime"`
}

type ChatMessageData struct {
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type ChatResponseData struct {
	Message   string `json:"message"`
	Model     string `json:"model,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type AGVEventData struct {
	EventType   string       `json:"event_type"`
	Explanation string       `json:"explanation"`
	Position    PositionData `json:"position,omitempty"`
	Timestamp   int64        `json:"timestamp"`
}
