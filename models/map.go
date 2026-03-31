package models

import "time"

const (
	CellEmpty    = 0
	CellObstacle = 1
	CellAGV      = 2
	CellEnemy    = 3
	CellPath     = 4
)

type Map struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	CellSize float64 `json:"cell_size"`

	Grid [][]int `json:"grid"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GridCoordinate struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type RealCoordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type MapUpdate struct {
	UpdateType    string           `json:"update_type"`
	AffectedCells []GridCoordinate `json:"affected_cells"`
	NewGrid       [][]int          `json:"new_grid"`
	Timestamp     time.Time        `json:"timestamp"`
}

type Obstacle struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Position   GridCoordinate `json:"position"`
	Size       int            `json:"size"`
	DetectedAt time.Time      `json:"detected_at"`
}

type PathPlanningRequest struct {
	Start        PositionData `json:"start"`
	Goal         PositionData `json:"goal"`
	Algorithm    string       `json:"algorithm"`
	AvoidEnemies bool         `json:"avoid_enemies"`
	MaxDistance   float64      `json:"max_distance"`
}

type PathPlanningResponse struct {
	Path          []PositionData `json:"path"`
	Length        float64        `json:"length"`
	EstimatedTime float64       `json:"estimated_time"`
	Algorithm     string        `json:"algorithm"`
	Success       bool          `json:"success"`
	Message       string        `json:"message"`
	CreatedAt     time.Time     `json:"created_at"`
}

type MapRegion struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	TopLeft     GridCoordinate `json:"top_left"`
	BottomRight GridCoordinate `json:"bottom_right"`
	Description string         `json:"description"`
}

type MapState struct {
	Map         Map          `json:"map"`
	AGVPosition PositionData `json:"agv_position"`
	Enemies     []Enemy      `json:"enemies"`
	Obstacles   []Obstacle   `json:"obstacles"`
	CurrentPath *PathData    `json:"current_path"`
	Timestamp   time.Time    `json:"timestamp"`
}
