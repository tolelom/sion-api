package algorithms

import "testing"

func TestFindPath_DoesNotCutBlockedCorners(t *testing.T) {
	g := NewGrid(2, 2)
	g.AddObstacle(1, 0)
	g.AddObstacle(0, 1)
	if path := g.FindPath(pt(0, 0), pt(1, 1)); path != nil {
		t.Fatalf("diagonal between two obstacles must be blocked, got %v", pathCoords(path))
	}
}

func TestFindPath_DiagonalNeedsBothSidesOpen(t *testing.T) {
	g := NewGrid(3, 3)
	g.AddObstacle(1, 0)
	path := g.FindPath(pt(0, 0), pt(1, 1))
	if len(path) != 3 {
		t.Fatalf("diagonal next to an obstacle must go around it, got %v", pathCoords(path))
	}
}

func TestFindPath_NegativeFractionalStartIsOutOfBounds(t *testing.T) {
	g := NewGrid(5, 5)
	if path := g.FindPath(Point{X: -0.9, Y: -0.9}, pt(3, 3)); path != nil {
		t.Fatalf("start left of the grid must be rejected, got %v", pathCoords(path))
	}
}
