package handlers

import (
	"net/http"
	"testing"
)

func TestHandlePathfinding_RejectsUnusableMapSizes(t *testing.T) {
	sizes := [][2]int{{-1, 2}, {0, 5}, {5, 0}, {100000, 100000}, {1 << 32, 1 << 32}}
	for _, s := range sizes {
		app := newPathfindingApp()
		body := map[string]any{
			"start":      map[string]float64{"x": 0, "y": 0},
			"goal":       map[string]float64{"x": 1, "y": 1},
			"map_width":  s[0],
			"map_height": s[1],
		}
		status, out := doPathfinding(t, app, body)
		if status != http.StatusBadRequest || out.Success {
			t.Errorf("map %dx%d: want 400, got %d success=%v", s[0], s[1], status, out.Success)
		}
	}
}
