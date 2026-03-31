package models

import "time"

const (
	EnemyTypeAhri     = "ahri"
	EnemyTypeObstacle = "obstacle"
)

const (
	EnemyStateAlive    = "alive"
	EnemyStateDefeated = "defeated"
	EnemyStateEscaped  = "escaped"
)

type Enemy struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	State string `json:"state"`

	Position   PositionData `json:"position"`
	Distance   float64      `json:"distance"`
	AngleToAGV float64      `json:"angle_to_agv"`

	HP    int     `json:"hp"`
	MaxHP int     `json:"max_hp"`
	Speed float64 `json:"speed"`

	FirstDetected  time.Time `json:"first_detected"`
	LastSeen       time.Time `json:"last_seen"`
	DetectionCount int       `json:"detection_count"`

	Priority    int    `json:"priority"`
	ThreatLevel string `json:"threat_level"`
}

type TargetSelection struct {
	SelectedEnemy *Enemy    `json:"selected_enemy"`
	Reason        string    `json:"reason"`
	Alternatives  []Enemy   `json:"alternatives"`
	Timestamp     time.Time `json:"timestamp"`
}

type EnemyGroup struct {
	Enemies       []Enemy   `json:"enemies"`
	TotalCount    int       `json:"total_count"`
	ActiveCount   int       `json:"active_count"`
	DefeatedCount int       `json:"defeated_count"`
	LastUpdate    time.Time `json:"last_update"`
}

type AttackResult struct {
	EnemyID     string    `json:"enemy_id"`
	Damage      int       `json:"damage"`
	RemainingHP int       `json:"remaining_hp"`
	Defeated    bool      `json:"defeated"`
	Timestamp   time.Time `json:"timestamp"`
}

type PriorityConfig struct {
	DistanceWeight float64 `json:"distance_weight"`
	HPWeight       float64 `json:"hp_weight"`
	ThreatWeight   float64 `json:"threat_weight"`
}

type TargetFoundEvent struct {
	Enemy           Enemy     `json:"enemy"`
	DetectionMethod string    `json:"detection_method"`
	Confidence      float64   `json:"confidence"`
	Timestamp       time.Time `json:"timestamp"`
}
