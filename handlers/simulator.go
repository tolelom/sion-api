package handlers

import (
	"sion-backend/services"

	"github.com/gofiber/fiber/v2"
)

func NewSimulatorStartHandler(sim *services.AGVSimulator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if sim.IsRunning() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "시뮬레이터가 이미 실행 중입니다",
			})
		}
		sim.Start()
		return c.JSON(fiber.Map{
			"success": true,
			"message": "AGV 시뮬레이터 시작",
		})
	}
}

func NewSimulatorStopHandler(sim *services.AGVSimulator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !sim.IsRunning() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "시뮬레이터가 실행 중이 아닙니다",
			})
		}
		sim.Stop()
		return c.JSON(fiber.Map{
			"success": true,
			"message": "AGV 시뮬레이터 중지",
		})
	}
}

func NewSimulatorStatusHandler(sim *services.AGVSimulator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status, enemies, mapW, mapH := sim.Snapshot()
		return c.JSON(fiber.Map{
			"success":   true,
			"running":   sim.IsRunning(),
			"agv_state": status,
			"enemies":   enemies,
			"map_size": fiber.Map{
				"width":  mapW,
				"height": mapH,
			},
		})
	}
}
