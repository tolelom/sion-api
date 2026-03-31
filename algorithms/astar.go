package algorithms

import (
	"fmt"
	"math"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Node struct {
	Point
	G      float64
	H      float64
	F      float64
	Parent *Node
}

type Grid struct {
	Width     int
	Height    int
	Obstacles map[string]bool
}

func NewGrid(width, height int) *Grid {
	return &Grid{
		Width:     width,
		Height:    height,
		Obstacles: make(map[string]bool),
	}
}

func (g *Grid) AddObstacle(x, y int) {
	key := fmt.Sprintf("%d,%d", x, y)
	g.Obstacles[key] = true
}

func (g *Grid) IsObstacle(x, y int) bool {
	key := fmt.Sprintf("%d,%d", x, y)
	return g.Obstacles[key]
}

func (g *Grid) IsValid(x, y int) bool {
	if x < 0 || y < 0 || x >= g.Width || y >= g.Height {
		return false
	}
	return !g.IsObstacle(x, y)
}

func heuristic(a, b Point) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func pointKey(p Point) string {
	return fmt.Sprintf("%.1f,%.1f", p.X, p.Y)
}

func (g *Grid) GetNeighbors(current Point) []Point {
	directions := []struct{ dx, dy int }{
		{0, 1}, {1, 0}, {0, -1}, {-1, 0},
		{1, 1}, {1, -1}, {-1, -1}, {-1, 1},
	}
	var neighbors []Point
	for _, d := range directions {
		nx := int(current.X) + d.dx
		ny := int(current.Y) + d.dy
		if g.IsValid(nx, ny) {
			neighbors = append(neighbors, Point{X: float64(nx), Y: float64(ny)})
		}
	}
	return neighbors
}

func (g *Grid) FindPath(start, goal Point) []Point {
	if start == goal {
		return []Point{start}
	}
	if g.IsObstacle(int(goal.X), int(goal.Y)) {
		return nil
	}
	openList := []*Node{
		{
			Point:  start,
			G:      0,
			H:      heuristic(start, goal),
			F:      heuristic(start, goal),
			Parent: nil,
		},
	}

	closedSet := make(map[string]bool)
	gScores := make(map[string]float64)
	gScores[pointKey(start)] = 0

	for len(openList) > 0 {
		// F 값 작은 노드 찾기
		currentIndex := 0
		for i := 1; i < len(openList); i++ {
			if openList[i].F < openList[currentIndex].F {
				currentIndex = i
			}
		}
		current := openList[currentIndex]

		if current.Point == goal {
			return reconstructPath(current)
		}

		openList = append(openList[:currentIndex], openList[currentIndex+1:]...)
		closedSet[pointKey(current.Point)] = true

		for _, neighbor := range g.GetNeighbors(current.Point) {
			key := pointKey(neighbor)
			if closedSet[key] {
				continue
			}

			moveCost := 1.0
			if current.X != neighbor.X && current.Y != neighbor.Y {
				moveCost = math.Sqrt2
			}
			tentativeG := current.G + moveCost

			if existingG, ok := gScores[key]; ok && tentativeG >= existingG {
				continue
			}

			neighborNode := &Node{
				Point:  neighbor,
				G:      tentativeG,
				H:      heuristic(neighbor, goal),
				F:      tentativeG + heuristic(neighbor, goal),
				Parent: current,
			}

			gScores[key] = tentativeG
			openList = append(openList, neighborNode)
		}
	}
	return nil
}

func reconstructPath(n *Node) []Point {
	var path []Point
	for n != nil {
		path = append([]Point{n.Point}, path...)
		n = n.Parent
	}
	return path
}
