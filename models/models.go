package models

import "time"

const (
	MessageTypePosition      = "position"
	MessageTypeStatus        = "status"
	MessageTypeMapUpdate     = "map_update"
	MessageTypeSystemInfo    = "system_info"
	MessageTypeError         = "error"
	MessageTypeCommand       = "command"
	MessageTypeModeChange    = "mode_change"
	MessageTypeEmergencyStop = "emergency_stop"
)

const (
	ModeAuto   = "auto"
	ModeManual = "manual"
)

const (
	StateIdle      = "idle"
	StateMoving    = "moving"
	StateCharging  = "charging"
	StateSearching = "searching"
	StateStopped   = "stopped"
	StateEmergency = "emergency"
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

type AGVStatus struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Position        PositionData `json:"position"`
	Mode            string       `json:"mode"`
	State           string       `json:"state"`
	Speed           float64      `json:"speed"`
	Battery         int          `json:"battery"`
	TargetEnemy     *Enemy       `json:"target_enemy"`
	DetectedEnemies []Enemy      `json:"detected_enemies"`
}

type Enemy struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	HP       int          `json:"hp"`
	Position PositionData `json:"position"`
}
