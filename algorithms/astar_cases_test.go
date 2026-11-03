package algorithms

import (
	"math"
	"reflect"
	"sion-backend/contract"
	"testing"
)

type expectedPath struct {
	Cost  float64  `json:"cost"`
	Cells int      `json:"cells"`
	Path  [][2]int `json:"path"`
}

type pathfindingCase struct {
	Name     string        `json:"name"`
	Map      []string      `json:"map"`
	Start    [2]int        `json:"start"`
	Goal     [2]int        `json:"goal"`
	Expected *expectedPath `json:"expected"`
}

func gridFromRows(rows []string) *Grid {
	g := NewGrid(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			if ch == '#' {
				g.AddObstacle(x, y)
			}
		}
	}
	return g
}

// sion-robot의 astar_normal과 같은 픽스처로 검사한다. 이 구현은 막힌 모서리를 대각으로 지나지 않으므로
// corner_cutting_expected는 보지 않는다.
func TestFindPath_SharedCases(t *testing.T) {
	var doc struct {
		Cases []pathfindingCase `json:"cases"`
	}
	contract.LoadJSON(t, "pathfinding-cases.json", &doc)
	if len(doc.Cases) == 0 {
		t.Fatal("no shared pathfinding cases")
	}
	for _, c := range doc.Cases {
		t.Run(c.Name, func(t *testing.T) {
			path := gridFromRows(c.Map).FindPath(pt(c.Start[0], c.Start[1]), pt(c.Goal[0], c.Goal[1]))
			if c.Expected == nil {
				if path != nil {
					t.Fatalf("want no path, got %v", pathCoords(path))
				}
				return
			}
			if path == nil {
				t.Fatal("want a path, got none")
			}
			coords := pathCoords(path)
			if coords[0] != c.Start || coords[len(coords)-1] != c.Goal {
				t.Fatalf("path %v does not run from %v to %v", coords, c.Start, c.Goal)
			}
			if len(path) != c.Expected.Cells || math.Abs(pathLength(path)-c.Expected.Cost) > 1e-6 {
				t.Fatalf("cells=%d cost=%.9f, want cells=%d cost=%.9f (%v)", len(path), pathLength(path), c.Expected.Cells, c.Expected.Cost, coords)
			}
			if c.Expected.Path != nil && !reflect.DeepEqual(coords, c.Expected.Path) {
				t.Fatalf("path %v, want %v", coords, c.Expected.Path)
			}
		})
	}
}
