package handlers

import (
	"log"
	"sion-backend/algorithms"

	"github.com/gofiber/fiber/v2"
)

type PathfindingRequest struct {
	Start struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"start"`
	Goal struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"goal"`
	MapWidth  int `json:"map_width"`
	MapHeight int `json:"map_height"`
	Obstacles []struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"obstacles"`
}

type PathfindingResponse struct {
	Success bool               `json:"success"`
	Path    []algorithms.Point `json:"path,omitempty"`
	Message string             `json:"message,omitempty"`
}

func HandlePathfinding(c *fiber.Ctx) error {
	var req PathfindingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(PathfindingResponse{
			Success: false,
			Message: "잘못된 요청 형식입니다",
		})
	}

	log.Printf("[INFO] 경로 탐색 요청: (%.1f,%.1f) -> (%.1f,%.1f), 맵=%dx%d, 장애물=%d개",
		req.Start.X, req.Start.Y, req.Goal.X, req.Goal.Y,
		req.MapWidth, req.MapHeight, len(req.Obstacles))

	grid := algorithms.NewGrid(req.MapWidth, req.MapHeight)
	for _, ob := range req.Obstacles {
		grid.AddObstacle(ob.X, ob.Y)
	}

	start := algorithms.Point{X: req.Start.X, Y: req.Start.Y}
	goal := algorithms.Point{X: req.Goal.X, Y: req.Goal.Y}

	path := grid.FindPath(start, goal)
	if path == nil {
		log.Printf("[WARN] 경로를 찾을 수 없음")
		return c.JSON(PathfindingResponse{
			Success: false,
			Message: "경로를 찾을 수 없습니다",
		})
	}

	log.Printf("[INFO] 경로 탐색 성공: %d개 웨이포인트", len(path))
	return c.JSON(PathfindingResponse{
		Success: true,
		Path:    path,
		Message: "경로 탐색 성공",
	})
}
