package models

import "time"

type ChatMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
}

type ChatRequest struct {
	Message   string `json:"message"`
	Context   string `json:"context,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type ChatResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
}

type AGVEventLog struct {
	Timestamp   time.Time    `json:"timestamp"`
	EventType   string       `json:"event_type"`
	Position    PositionData `json:"position"`
	Enemies     []Enemy      `json:"enemies,omitempty"`
	Decision    string       `json:"decision"`
	Explanation string       `json:"explanation"`
}

type LLMContext struct {
	CurrentPosition PositionData  `json:"current_position"`
	TargetEnemy     *Enemy        `json:"target_enemy,omitempty"`
	Enemies         []Enemy       `json:"enemies"`
	CurrentMode     string        `json:"current_mode"`
	BatteryLevel    float64       `json:"battery_level"`
	Speed           float64       `json:"speed"`
	RecentEvents    []AGVEventLog `json:"recent_events"`
}
