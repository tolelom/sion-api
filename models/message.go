package models

import "time"

// ========================================
// 메시지 타입 상수
// ========================================
const (
	// AGV → Server → Web
	MessageTypePosition    = "position"     // AGV 위치 업데이트
	MessageTypeStatus      = "status"       // AGV 상태 업데이트
	MessageTypeLog         = "log"          // 행동 로그
	MessageTypeTargetFound = "target_found" // 적 발견
	MessageTypePathUpdate  = "path_update"  // 경로 업데이트

	// Web → Server → AGV
	MessageTypeCommand       = "command"        // 이동/정지 명령
	MessageTypeModeChange    = "mode_change"    // 자동/수동 모드 전환
	MessageTypeEmergencyStop = "emergency_stop" // 긴급 정지

	// 🆕 채팅 관련
	MessageTypeChat         = "chat"          // Web → Server (사용자 질문)
	MessageTypeChatResponse = "chat_response" // Server → Web (AI 응답)
	MessageTypeAGVEvent     = "agv_event"     // Server → Web (AGV 이벤트 설명)

	// LLM → Server → Web
	MessageTypeLLMExplanation = "llm_explanation" // AI 설명
	MessageTypeTTS            = "tts"             // 음성 중계

	// Server → All
	MessageTypeMapUpdate  = "map_update"  // 맵 업데이트
	MessageTypeSystemInfo = "system_info" // 시스템 정보

	// 연결 상태
	MessageTypeAGVConnected    = "agv_connected"    // AGV 연결됨
	MessageTypeAGVDisconnected = "agv_disconnected" // AGV 연결 끊김

	// 에러
	MessageTypeError = "error" // 에러 메시지
)

// ========================================
// 공통 WebSocket 메시지 형식
// ========================================
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"` // Unix timestamp (ms)
}

// ========================================
// AGV 위치 데이터
// ========================================
type PositionData struct {
	X         float64   `json:"x"`         // 현재 X 좌표 (미터)
	Y         float64   `json:"y"`         // 현재 Y 좌표 (미터)
	Angle     float64   `json:"angle"`     // 현재 각도 (라디안)
	Timestamp time.Time `json:"timestamp"` // 측정 시각
}

// ========================================
// 명령 메시지
// ========================================

// 이동 명령
type MoveCommand struct {
	TargetX float64 `json:"target_x"` // 목표 X 좌표
	TargetY float64 `json:"target_y"` // 목표 Y 좌표
	Mode    string  `json:"mode"`     // "direct" | "pathfinding"
}

// 모드 변경 명령
type ModeChangeCommand struct {
	Mode string `json:"mode"` // "auto" | "manual"
}

// 긴급 정지 명령
type EmergencyStopCommand struct {
	Reason string `json:"reason"` // 정지 사유
}

// ========================================
// 경로 데이터
// ========================================
type PathData struct {
	Points    []PositionData `json:"points"`     // 경로 포인트 리스트
	Length    float64        `json:"length"`     // 전체 경로 길이
	Algorithm string         `json:"algorithm"`  // "a_star" | "dijkstra"
	CreatedAt time.Time      `json:"created_at"` // 경로 생성 시각
}

// ========================================
// LLM 설명 데이터
// ========================================
type LLMExplanation struct {
	Text      string    `json:"text"`      // 설명 텍스트
	Action    string    `json:"action"`    // 현재 행동 ("moving", "attacking", "searching")
	Reason    string    `json:"reason"`    // 행동 이유
	Timestamp time.Time `json:"timestamp"` // 생성 시각
}

// ========================================
// TTS 데이터
// ========================================
type TTSData struct {
	Text     string `json:"text"`      // 음성 변환할 텍스트
	AudioURL string `json:"audio_url"` // 음성 파일 URL (옵션)
	Voice    string `json:"voice"`     // 음성 타입 (옵션)
}

// ========================================
// 시스템 정보
// ========================================
type SystemInfo struct {
	ConnectedClients int       `json:"connected_clients"` // 연결된 클라이언트 수
	AGVConnected     bool      `json:"agv_connected"`     // AGV 연결 상태
	ServerTime       time.Time `json:"server_time"`       // 서버 시각
	Uptime           int64     `json:"uptime"`            // 가동 시간 (초)
}

// ========================================
// 🆕 채팅 메시지 데이터
// ========================================

// ChatMessageData - 사용자 채팅 메시지
type ChatMessageData struct {
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// ChatResponseData - AI 응답 데이터
type ChatResponseData struct {
	Message   string `json:"message"`
	Model     string `json:"model,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// AGVEventData - AGV 이벤트 설명 데이터
type AGVEventData struct {
	EventType   string       `json:"event_type"` // "target_change", "path_start", "charging", "avoid_obstacle"
	Explanation string       `json:"explanation"`
	Position    PositionData `json:"position,omitempty"`
	Timestamp   int64        `json:"timestamp"`
}
