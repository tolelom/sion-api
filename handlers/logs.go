package handlers

import (
	"sion-backend/services"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetRecentLogs - 최근 로그 조회
func HandleGetRecentLogs(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001") // 기본 AGV ID
	limitStr := c.Query("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	// 최근 로그 조회
	logs, err := services.GetRecentLogs(agvID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch logs",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"count":   len(logs),
		"logs":    logs,
	})
}

// HandleGetLogsByTimeRange - 시간 범위로 로그 조회
func HandleGetLogsByTimeRange(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	startStr := c.Query("start") // RFC3339 format
	endStr := c.Query("end")     // RFC3339 format
	limitStr := c.Query("limit", "100")

	// 시작 시간 파싱
	var start time.Time
	if startStr != "" {
		parsed, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid start time format (use RFC3339)",
			})
		}
		start = parsed
	} else {
		// 기본: 24시간 전
		start = time.Now().Add(-24 * time.Hour)
	}

	// 종료 시간 파싱
	var end time.Time
	if endStr != "" {
		parsed, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid end time format (use RFC3339)",
			})
		}
		end = parsed
	} else {
		// 기본: 현재 시간
		end = time.Now()
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	// 로그 조회
	logs, err := services.GetLogsByTimeRange(agvID, start, end, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch logs",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"count":      len(logs),
		"time_range": fiber.Map{
			"start": start.Format(time.RFC3339),
			"end":   end.Format(time.RFC3339),
		},
		"logs": logs,
	})
}

// HandleGetLogsByEventType - 이벤트 타입별 로그 조회
func HandleGetLogsByEventType(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	eventType := c.Query("event_type")
	limitStr := c.Query("limit", "100")

	if eventType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event_type parameter is required",
		})
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	// 로그 조회
	logs, err := services.GetLogsByEventType(agvID, eventType, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch logs",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"count":      len(logs),
		"event_type": eventType,
		"logs":       logs,
	})
}

// HandleGetLogStats - 로그 통계 조회
func HandleGetLogStats(c *fiber.Ctx) error {
	agvID := c.Query("agv_id", "sion-001")
	hoursStr := c.Query("hours", "24")

	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	// 통계 조회
	stats, err := services.GetLogStats(agvID, hours)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch stats",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}
