package services

import (
	"fmt"
	"sion-backend/models"
	"time"
)

func GetRecentLogs(agvID string, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	err := db.Where("agv_id = ?", agvID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func GetLogsByTimeRange(agvID string, start, end time.Time, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	query := db.Where("agv_id = ? AND created_at BETWEEN ? AND ?", agvID, start, end)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func GetLogsByEventType(agvID string, eventType string, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	err := db.Where("agv_id = ? AND event_type = ?", agvID, eventType).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func GetLogStats(agvID string, hours int) (map[string]interface{}, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var totalLogs int64
	if err := db.Model(&models.AGVLog{}).
		Where("agv_id = ? AND created_at >= ?", agvID, since).
		Count(&totalLogs).Error; err != nil {
		return nil, fmt.Errorf("log count 실패: %w", err)
	}

	var eventCounts []struct {
		EventType string
		Count     int64
	}
	if err := db.Model(&models.AGVLog{}).
		Select("event_type, COUNT(*) as count").
		Where("agv_id = ? AND created_at >= ?", agvID, since).
		Group("event_type").
		Scan(&eventCounts).Error; err != nil {
		return nil, fmt.Errorf("event_type 집계 실패: %w", err)
	}

	eventMap := make(map[string]int64)
	for _, ec := range eventCounts {
		eventMap[ec.EventType] = ec.Count
	}

	return map[string]interface{}{
		"total_logs":   totalLogs,
		"event_counts": eventMap,
		"time_range":   fmt.Sprintf("Last %d hours", hours),
	}, nil
}
