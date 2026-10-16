package models

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Waypoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
