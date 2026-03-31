package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sion-backend/models"
	"sync"
	"time"
)

type retryableLog struct {
	Log        models.AGVLog
	RetryCount int
}

const maxRetries = 3
const maxFailedLogs = 500

type LogBuffer struct {
	logs       []models.AGVLog
	failedLogs []retryableLog
	mu         sync.Mutex
	flushSize  int           // 일괄 저장 크기
	flushTime  time.Duration // 자동 플러시 시간
	stopChan   chan bool
}

var logBuffer *LogBuffer

func InitLogging(flushSize int, flushInterval time.Duration) {
	logBuffer = &LogBuffer{
		logs:       make([]models.AGVLog, 0, flushSize*2),
		failedLogs: make([]retryableLog, 0),
		flushSize:  flushSize,
		flushTime:  flushInterval,
		stopChan:   make(chan bool),
	}

	go logBuffer.autoFlush()

	log.Printf("[INFO] 로깅 시스템 초기화 (flushSize=%d, interval=%v)", flushSize, flushInterval)
}

func (lb *LogBuffer) autoFlush() {
	ticker := time.NewTicker(lb.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.Flush()
		case <-lb.stopChan:
			lb.Flush() // 종료 시 남은 로그 저장
			return
		}
	}
}

func AddLog(logEntry models.AGVLog) {
	if logBuffer == nil {
		log.Println("[WARN] 로깅 시스템이 초기화되지 않음")
		return
	}

	logBuffer.mu.Lock()
	logBuffer.logs = append(logBuffer.logs, logEntry)
	size := len(logBuffer.logs)
	logBuffer.mu.Unlock()

	if size >= logBuffer.flushSize {
		go logBuffer.Flush()
	}
}

func (lb *LogBuffer) Flush() {
	if db == nil {
		return
	}

	lb.mu.Lock()
	if len(lb.logs) == 0 && len(lb.failedLogs) == 0 {
		lb.mu.Unlock()
		return
	}

	var retryLogs []retryableLog
	if len(lb.failedLogs) > 0 {
		retryLogs = make([]retryableLog, len(lb.failedLogs))
		copy(retryLogs, lb.failedLogs)
		lb.failedLogs = lb.failedLogs[:0]
	}

	var newLogs []models.AGVLog
	if len(lb.logs) > 0 {
		newLogs = make([]models.AGVLog, len(lb.logs))
		copy(newLogs, lb.logs)
		lb.logs = lb.logs[:0]
	}
	lb.mu.Unlock()

	var stillFailed []retryableLog
	if len(retryLogs) > 0 {
		retryBatch := make([]models.AGVLog, len(retryLogs))
		for i, rl := range retryLogs {
			retryBatch[i] = rl.Log
		}
		if err := db.CreateInBatches(retryBatch, 100).Error; err != nil {
			log.Printf("[ERROR] 재시도 로그 저장 실패: %v", err)
			for _, rl := range retryLogs {
				rl.RetryCount++
				if rl.RetryCount >= maxRetries {
					log.Printf("[WARN] 로그 재시도 %d회 초과, 폐기", maxRetries)
				} else {
					stillFailed = append(stillFailed, rl)
				}
			}
		} else {
			log.Printf("[INFO] 재시도 로그 %d개 저장 완료", len(retryBatch))
		}
	}

	if len(newLogs) > 0 {
		if err := db.CreateInBatches(newLogs, 100).Error; err != nil {
			log.Printf("[ERROR] 로그 저장 실패: %v", err)
			for _, l := range newLogs {
				stillFailed = append(stillFailed, retryableLog{Log: l, RetryCount: 0})
			}
		} else {
			log.Printf("[INFO] 로그 %d개 저장 완료", len(newLogs))
		}
	}

	if len(stillFailed) > 0 {
		lb.mu.Lock()
		lb.failedLogs = append(lb.failedLogs, stillFailed...)
		if len(lb.failedLogs) > maxFailedLogs {
			dropped := len(lb.failedLogs) - maxFailedLogs
			lb.failedLogs = lb.failedLogs[dropped:]
			log.Printf("[WARN] 실패 로그 큐 초과, %d개 폐기", dropped)
		}
		lb.mu.Unlock()
	}
}

func LogAGVPosition(agvID string, position models.PositionData) {
	logEntry := models.AGVLog{
		CreatedAt:     time.Now(),
		EventType:     "position_update",
		MessageType:   "position",
		AGVID:         agvID,
		PositionX:     position.X,
		PositionY:     position.Y,
		PositionAngle: position.Angle,
	}
	AddLog(logEntry)
}

func LogAGVStatus(agvID string, status *models.AGVStatus) {
	logEntry := models.AGVLog{
		CreatedAt:     time.Now(),
		EventType:     "status_update",
		MessageType:   "status",
		AGVID:         agvID,
		PositionX:     status.Position.X,
		PositionY:     status.Position.Y,
		PositionAngle: status.Position.Angle,
		Speed:         status.Speed,
		Battery:       status.Battery,
		Mode:          status.Mode,
		State:         status.State,
	}

	if status.TargetEnemy != nil {
		logEntry.TargetEnemyID = status.TargetEnemy.ID
		logEntry.TargetEnemyHP = status.TargetEnemy.HP
		logEntry.TargetEnemyName = status.TargetEnemy.Name
	}

	AddLog(logEntry)
}

func LogTargetFound(agvID string, target *models.Enemy) {
	logEntry := models.AGVLog{
		CreatedAt:       time.Now(),
		EventType:       "target_found",
		MessageType:     "target_found",
		AGVID:           agvID,
		TargetEnemyID:   target.ID,
		TargetEnemyHP:   target.HP,
		TargetEnemyName: target.Name,
	}
	AddLog(logEntry)
}

func LogCommand(agvID string, commandType string, targetX, targetY float64) {
	logEntry := models.AGVLog{
		CreatedAt:   time.Now(),
		EventType:   "command",
		MessageType: "command",
		AGVID:       agvID,
		CommandType: commandType,
		TargetX:     targetX,
		TargetY:     targetY,
	}
	AddLog(logEntry)
}

func LogAIExplanation(agvID string, eventType string, explanation string) {
	logEntry := models.AGVLog{
		CreatedAt:     time.Now(),
		EventType:     eventType,
		MessageType:   "agv_event",
		AGVID:         agvID,
		AIExplanation: explanation,
	}
	AddLog(logEntry)
}

func LogWebSocketMessage(agvID string, msg models.WebSocketMessage) {
	dataJSON, _ := json.Marshal(msg.Data)

	logEntry := models.AGVLog{
		CreatedAt:   time.Now(),
		EventType:   inferEventType(msg.Type),
		MessageType: msg.Type,
		AGVID:       agvID,
		DataJSON:    string(dataJSON),
	}

	extractLogData(&logEntry, msg)

	AddLog(logEntry)
}

func extractLogData(logEntry *models.AGVLog, msg models.WebSocketMessage) {
	dataMap, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	// 위치 데이터
	if x, ok := dataMap["x"].(float64); ok {
		logEntry.PositionX = x
	}
	if y, ok := dataMap["y"].(float64); ok {
		logEntry.PositionY = y
	}
	if angle, ok := dataMap["angle"].(float64); ok {
		logEntry.PositionAngle = angle
	}

	// 상태 데이터
	if speed, ok := dataMap["speed"].(float64); ok {
		logEntry.Speed = speed
	}
	if battery, ok := dataMap["battery"].(float64); ok {
		logEntry.Battery = int(battery)
	}
	if mode, ok := dataMap["mode"].(string); ok {
		logEntry.Mode = mode
	}
	if state, ok := dataMap["state"].(string); ok {
		logEntry.State = state
	}

	// 명령 데이터
	if targetX, ok := dataMap["target_x"].(float64); ok {
		logEntry.TargetX = targetX
	}
	if targetY, ok := dataMap["target_y"].(float64); ok {
		logEntry.TargetY = targetY
	}
	if action, ok := dataMap["action"].(string); ok {
		logEntry.CommandType = action
	}
}

func inferEventType(msgType string) string {
	switch msgType {
	case "position":
		return "position_update"
	case "status":
		return "status_update"
	case "target_found":
		return "target_detected"
	case "chat":
		return "user_question"
	case "command":
		return "command_received"
	case "chat_response":
		return "ai_response"
	case "agv_event":
		return "event_description"
	case "emergency_stop":
		return "emergency_stop"
	case "mode_change":
		return "mode_change"
	default:
		return msgType
	}
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
	db.Model(&models.AGVLog{}).
		Where("agv_id = ? AND created_at >= ?", agvID, since).
		Count(&totalLogs)

	// 이벤트 타입별 카운트
	var eventCounts []struct {
		EventType string
		Count     int64
	}
	db.Model(&models.AGVLog{}).
		Select("event_type, COUNT(*) as count").
		Where("agv_id = ? AND created_at >= ?", agvID, since).
		Group("event_type").
		Scan(&eventCounts)

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

func StopLogging() {
	if logBuffer != nil {
		logBuffer.stopChan <- true
		log.Println("[INFO] 로깅 시스템 종료")
	}
}
