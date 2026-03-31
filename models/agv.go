package models

import "time"

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

type AGVStatus struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Connected  bool      `json:"connected"`
	LastUpdate time.Time `json:"last_update"`

	Position PositionData `json:"position"`

	Mode    string  `json:"mode"`
	State   string  `json:"state"`
	Speed   float64 `json:"speed"`
	Battery int     `json:"battery"`

	CurrentPath *PathData     `json:"current_path"`
	TargetPos   *PositionData `json:"target_pos"`

	TargetEnemy     *Enemy  `json:"target_enemy"`
	DetectedEnemies []Enemy `json:"detected_enemies"`

	Sensors SensorData `json:"sensors"`
}

type SensorData struct {
	FrontDistance float64 `json:"front_distance"`
	LeftDistance  float64 `json:"left_distance"`
	RightDistance float64 `json:"right_distance"`

	AccelX float64 `json:"accel_x"`
	AccelY float64 `json:"accel_y"`
	AccelZ float64 `json:"accel_z"`
	GyroX  float64 `json:"gyro_x"`
	GyroY  float64 `json:"gyro_y"`
	GyroZ  float64 `json:"gyro_z"`

	CameraActive    bool `json:"camera_active"`
	ObjectsDetected int  `json:"objects_detected"`
}

type MotorControl struct {
	LeftSpeed  float64 `json:"left_speed"`
	RightSpeed float64 `json:"right_speed"`
	Duration   int     `json:"duration"`
}

type AGVStats struct {
	TotalDistance    float64   `json:"total_distance"`
	TotalTime       int64     `json:"total_time"`
	EnemiesDefeated int       `json:"enemies_defeated"`
	Collisions      int       `json:"collisions"`
	StartTime       time.Time `json:"start_time"`
}
