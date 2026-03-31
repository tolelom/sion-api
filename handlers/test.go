package handlers

import (
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

func NewTestPositionHandler(br *services.Broker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		msg := models.WebSocketMessage{
			Type: models.MessageTypePosition,
			Data: models.PositionData{
				X:         10.5,
				Y:         15.2,
				Angle:     1.57,
				Timestamp: time.Now(),
			},
			Timestamp: time.Now().UnixMilli(),
		}

		br.BroadcastToWeb(msg)
		services.LogAGVPosition("sion-001", msg.Data.(models.PositionData))

		return c.JSON(fiber.Map{
			"success": true,
			"message": "테스트 데이터 전송 성공",
		})
	}
}

func NewTestStatusHandler(br *services.Broker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		msg := models.WebSocketMessage{
			Type: models.MessageTypeStatus,
			Data: map[string]interface{}{
				"battery": 85,
				"speed":   1.5,
				"mode":    "auto",
				"state":   "moving",
			},
			Timestamp: time.Now().UnixMilli(),
		}

		br.BroadcastToWeb(msg)
		services.LogWebSocketMessage("sion-001", msg)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "상태 데이터 전송 성공",
		})
	}
}

func NewTestEventHandler(br *services.Broker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		testStatus := &models.AGVStatus{
			ID:   "sion-001",
			Name: "사이온",
			Position: models.PositionData{
				X: 10.5, Y: 15.2, Angle: 0.785,
			},
			Mode:    models.ModeAuto,
			State:   models.StateCharging,
			Speed:   2.5,
			Battery: 85,
			TargetEnemy: &models.Enemy{
				ID:       "enemy-1",
				Name:     "아리",
				HP:       30,
				Position: models.PositionData{X: 15, Y: 12},
			},
			DetectedEnemies: []models.Enemy{
				{ID: "enemy-1", Name: "아리", HP: 30, Position: models.PositionData{X: 15, Y: 12}},
				{ID: "enemy-2", Name: "아리", HP: 80, Position: models.PositionData{X: 20, Y: 18}},
			},
		}

		services.LogAGVStatus("sion-001", testStatus)
		if testStatus.TargetEnemy != nil {
			services.LogTargetFound("sion-001", testStatus.TargetEnemy)
		}
		ExplainAGVEvent("target_change", testStatus)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "이벤트 설명 생성 중...",
		})
	}
}
