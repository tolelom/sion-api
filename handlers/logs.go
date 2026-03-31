package handlers

import (
	"sion-backend/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func HandleGetRecentLogs(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	limitStr := c.Query("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	logs, err := services.GetRecentLogs(agvID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "로그 조회 실패",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"count":   len(logs),
		"logs":    logs,
	})
}

func HandleGetLogsByTimeRange(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	startStr := c.Query("start")
	endStr := c.Query("end")
	limitStr := c.Query("limit", "100")

	var start time.Time
	if startStr != "" {
		parsed, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "잘못된 start 시간 형식 (RFC3339 사용)",
			})
		}
		start = parsed
	} else {
		start = time.Now().Add(-24 * time.Hour)
	}

	var end time.Time
	if endStr != "" {
		parsed, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "잘못된 end 시간 형식 (RFC3339 사용)",
			})
		}
		end = parsed
	} else {
		end = time.Now()
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	logs, err := services.GetLogsByTimeRange(agvID, start, end, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "로그 조회 실패",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"count":   len(logs),
		"time_range": fiber.Map{
			"start": start.Format(time.RFC3339),
			"end":   end.Format(time.RFC3339),
		},
		"logs": logs,
	})
}

func HandleGetLogsByEventType(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	eventType := c.Query("event_type")
	limitStr := c.Query("limit", "100")

	if eventType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event_type 파라미터가 필요합니다",
		})
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	logs, err := services.GetLogsByEventType(agvID, eventType, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "로그 조회 실패",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"count":      len(logs),
		"event_type": eventType,
		"logs":       logs,
	})
}

func HandleGetLogStats(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	hoursStr := c.Query("hours", "24")

	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	stats, err := services.GetLogStats(agvID, hours)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "통계 조회 실패",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}
