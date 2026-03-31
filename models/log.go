package models

import (
	"time"
)

type AGVLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	EventType   string `json:"event_type"`
	MessageType string `json:"message_type"`

	AGVID         string  `json:"agv_id"`
	PositionX     float64 `json:"position_x"`
	PositionY     float64 `json:"position_y"`
	PositionAngle float64 `json:"position_angle"`
	Speed         float64 `json:"speed"`
	Battery       int     `json:"battery"`
	Mode          string  `json:"mode"`
	State         string  `json:"state"`

	TargetEnemyID   string `json:"target_enemy_id"`
	TargetEnemyHP   int    `json:"target_enemy_hp"`
	TargetEnemyName string `json:"target_enemy_name"`

	CommandType string  `json:"command_type"`
	TargetX     float64 `json:"target_x"`
	TargetY     float64 `json:"target_y"`

	AIExplanation string `json:"ai_explanation"`

	DataJSON string `json:"data_json"`
	UserID   string `json:"user_id"`
}

type LogSummary struct {
	RecentActions []AGVLog `json:"recent_actions"`
	Patterns      string   `json:"patterns"`
}
