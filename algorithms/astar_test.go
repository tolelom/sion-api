package algorithms

import (
	"math"
	"testing"
)

func pt(x, y int) Point {
	return Point{X: float64(x), Y: float64(y)}
}

func pathCoords(p []Point) [][2]int {
	out := make([][2]int, len(p))
	for i, q := range p {
		out[i] = [2]int{int(q.X), int(q.Y)}
	}
	return out
}

func pathLength(p []Point) float64 {
	if len(p) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(p); i++ {
		dx := p[i].X - p[i-1].X
		dy := p[i].Y - p[i-1].Y
		total += math.Sqrt(dx*dx + dy*dy)
	}
	return total
}

func TestFindPath_StartEqualsGoal(t *testing.T) {
	g := NewGrid(5, 5)
	path := g.FindPath(pt(2, 2), pt(2, 2))
	if len(path) != 1 || path[0] != pt(2, 2) {
		t.Fatalf("start==goal에서 [start] 기대, got %v", path)
	}
}

func TestFindPath_StraightLine(t *testing.T) {
	g := NewGrid(5, 5)
	path := g.FindPath(pt(0, 0), pt(4, 0))
	coords := pathCoords(path)
	want := [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}}
	if len(coords) != len(want) {
		t.Fatalf("길이 %d 기대, got %d (%v)", len(want), len(coords), coords)
	}
	for i := range want {
		if coords[i] != want[i] {
			t.Fatalf("idx %d: %v 기대, got %v", i, want[i], coords[i])
		}
	}
}

func TestFindPath_DiagonalShortcut(t *testing.T) {
	g := NewGrid(5, 5)
	path := g.FindPath(pt(0, 0), pt(4, 4))
	if len(path) != 5 {
		t.Fatalf("대각 5칸 기대, got %d (%v)", len(path), pathCoords(path))
	}
	expectLen := 4 * math.Sqrt2
	if math.Abs(pathLength(path)-expectLen) > 1e-9 {
		t.Fatalf("길이 %.6f 기대, got %.6f", expectLen, pathLength(path))
	}
}

func TestFindPath_GoalOnObstacle(t *testing.T) {
	g := NewGrid(5, 5)
	g.AddObstacle(3, 3)
	if path := g.FindPath(pt(0, 0), pt(3, 3)); path != nil {
		t.Fatalf("장애물 위 목표는 nil 기대, got %v", path)
	}
}

func TestFindPath_StartOnObstacle(t *testing.T) {
	g := NewGrid(5, 5)
	g.AddObstacle(0, 0)
	if path := g.FindPath(pt(0, 0), pt(2, 2)); path != nil {
		t.Fatalf("장애물 위 시작은 nil 기대, got %v", path)
	}
}

func TestFindPath_OutOfBounds(t *testing.T) {
	g := NewGrid(5, 5)
	if path := g.FindPath(pt(0, 0), pt(10, 10)); path != nil {
		t.Fatalf("범위 밖 목표는 nil 기대, got %v", path)
	}
	if path := g.FindPath(pt(-1, 0), pt(2, 2)); path != nil {
		t.Fatalf("범위 밖 시작은 nil 기대, got %v", path)
	}
}

func TestFindPath_WallAvoidance(t *testing.T) {
	g := NewGrid(5, 5)
	// 세로 벽 (x=2, y=0..3)을 두고 (0,2)→(4,2) 우회
	for y := 0; y <= 3; y++ {
		g.AddObstacle(2, y)
	}
	path := g.FindPath(pt(0, 2), pt(4, 2))
	if path == nil {
		t.Fatal("우회 경로 기대, got nil")
	}
	for _, p := range path {
		x, y := int(p.X), int(p.Y)
		if x == 2 && y >= 0 && y <= 3 {
			t.Fatalf("벽을 통과: %v", pathCoords(path))
		}
	}
	if path[0] != pt(0, 2) || path[len(path)-1] != pt(4, 2) {
		t.Fatalf("끝점 불일치: %v", pathCoords(path))
	}
}

func TestFindPath_Unreachable(t *testing.T) {
	g := NewGrid(5, 5)
	// 목표(4,4)를 ㄴ자 벽으로 봉쇄
	g.AddObstacle(3, 4)
	g.AddObstacle(4, 3)
	g.AddObstacle(3, 3)
	if path := g.FindPath(pt(0, 0), pt(4, 4)); path != nil {
		t.Fatalf("도달 불가능, got %v", pathCoords(path))
	}
}

func TestFindPath_FloatInputNormalized(t *testing.T) {
	// 부동소수점 좌표가 들어와도 셀 단위로 정규화되어 동작해야 한다.
	g := NewGrid(5, 5)
	path := g.FindPath(Point{X: 0.4, Y: 0.6}, Point{X: 4.2, Y: 4.9})
	if len(path) == 0 {
		t.Fatal("부동소수점 입력에서도 경로 기대")
	}
	if path[0] != pt(0, 0) || path[len(path)-1] != pt(4, 4) {
		t.Fatalf("정수 정규화 후 (0,0)→(4,4) 기대, got %v", pathCoords(path))
	}
}

func TestFindPath_LargeGrid(t *testing.T) {
	const N = 100
	g := NewGrid(N, N)
	// 중앙에 세로 벽 하나 (틈 한 칸)
	for y := 0; y < N; y++ {
		if y == N/2 {
			continue
		}
		g.AddObstacle(N/2, y)
	}
	path := g.FindPath(pt(0, 0), pt(N-1, N-1))
	if path == nil {
		t.Fatal("100x100 우회 경로 기대")
	}
	if path[0] != pt(0, 0) || path[len(path)-1] != pt(N-1, N-1) {
		t.Fatalf("끝점 불일치")
	}
}
