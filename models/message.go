package models

import "time"

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

type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
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
