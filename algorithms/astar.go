package algorithms

import (
	"container/heap"
	"math"
)

// Point는 외부 API 호환을 위해 float64 좌표를 유지한다.
// 내부 그리드는 입력을 int로 캐스팅해 셀 단위로 다룬다.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Grid struct {
	Width     int
	Height    int
	obstacles []bool // y*Width + x
}

func NewGrid(width, height int) *Grid {
	return &Grid{
		Width:     width,
		Height:    height,
		obstacles: make([]bool, width*height),
	}
}

func (g *Grid) idx(x, y int) int {
	return y*g.Width + x
}

func (g *Grid) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < g.Width && y < g.Height
}

func (g *Grid) AddObstacle(x, y int) {
	if g.inBounds(x, y) {
		g.obstacles[g.idx(x, y)] = true
	}
}

func (g *Grid) IsObstacle(x, y int) bool {
	if !g.inBounds(x, y) {
		return false
	}
	return g.obstacles[g.idx(x, y)]
}

func (g *Grid) IsValid(x, y int) bool {
	return g.inBounds(x, y) && !g.obstacles[g.idx(x, y)]
}

// 8방향 이동 (dx, dy)
var directions8 = [8][2]int{
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
}

func heuristic(ax, ay, bx, by int) float64 {
	dx := float64(ax - bx)
	dy := float64(ay - by)
	return math.Sqrt(dx*dx + dy*dy)
}

type pqItem struct {
	x, y int
	f, g float64
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].f < pq[j].f }
func (pq priorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x any) { *pq = append(*pq, x.(*pqItem)) }
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	*pq = old[:n-1]
	return it
}

// FindPath는 8방향 그리드에서 start→goal 최단 경로를 찾아 반환한다.
// 좌표는 입력 시 int로 캐스팅되어 셀 단위로 처리된다.
//   - start == goal (같은 셀, 통과 가능): [start] 반환
//   - 경로 없음 / start·goal이 범위 밖이거나 장애물: nil 반환
func (g *Grid) FindPath(start, goal Point) []Point {
	sx, sy := int(start.X), int(start.Y)
	gx, gy := int(goal.X), int(goal.Y)

	if !g.IsValid(sx, sy) || !g.IsValid(gx, gy) {
		return nil
	}
	if sx == gx && sy == gy {
		return []Point{{X: float64(sx), Y: float64(sy)}}
	}

	W := g.Width
	total := W * g.Height

	gScore := make([]float64, total)
	parent := make([]int, total)
	closed := make([]bool, total)
	for i := range gScore {
		gScore[i] = math.Inf(1)
		parent[i] = -1
	}

	startIdx := g.idx(sx, sy)
	goalIdx := g.idx(gx, gy)
	gScore[startIdx] = 0

	pq := &priorityQueue{}
	heap.Push(pq, &pqItem{x: sx, y: sy, f: heuristic(sx, sy, gx, gy), g: 0})

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(*pqItem)
		curIdx := g.idx(cur.x, cur.y)

		// 동일 셀에 대한 stale 항목은 건너뜀
		if closed[curIdx] {
			continue
		}
		closed[curIdx] = true

		if curIdx == goalIdx {
			return reconstructPath(parent, startIdx, goalIdx, W)
		}

		for _, d := range directions8 {
			nx, ny := cur.x+d[0], cur.y+d[1]
			if !g.IsValid(nx, ny) {
				continue
			}
			nIdx := g.idx(nx, ny)
			if closed[nIdx] {
				continue
			}

			step := 1.0
			if d[0] != 0 && d[1] != 0 {
				step = math.Sqrt2
			}
			tentativeG := cur.g + step

			if tentativeG >= gScore[nIdx] {
				continue
			}
			gScore[nIdx] = tentativeG
			parent[nIdx] = curIdx
			heap.Push(pq, &pqItem{
				x: nx,
				y: ny,
				f: tentativeG + heuristic(nx, ny, gx, gy),
				g: tentativeG,
			})
		}
	}
	return nil
}

func reconstructPath(parent []int, startIdx, goalIdx, width int) []Point {
	path := make([]Point, 0, 32)
	for cur := goalIdx; cur != -1; cur = parent[cur] {
		x := cur % width
		y := cur / width
		path = append(path, Point{X: float64(x), Y: float64(y)})
		if cur == startIdx {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
