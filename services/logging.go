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

	// 자동 플러시 고루틴 시작
	go logBuffer.autoFlush()

	log.Printf("✅ 로깅 시스템 초기화 완료 (flushSize: %d, flushInterval: %v)", flushSize, flushInterval)
}

// autoFlush - 주기적 로그 저장
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

// AddLog - 로그 버퍼에 추가 (비동기)
func AddLog(logEntry models.AGVLog) {
	if logBuffer == nil {
		log.Println("⚠️ 로깅 시스템이 초기화되지 않음")
		return
	}

	logBuffer.mu.Lock()
	logBuffer.logs = append(logBuffer.logs, logEntry)
	size := len(logBuffer.logs)
	logBuffer.mu.Unlock()

	// 버퍼 크기가 차면 즉시 플러시
	if size >= logBuffer.flushSize {
		go logBuffer.Flush()
	}
}

// Flush - 버퍼의 모든 로그를 DB에 저장 (실패 시 재시도 큐로 이동)
func (lb *LogBuffer) Flush() {
	if db == nil {
		return // DB 없으면 버퍼 유지 (데이터 유실 방지)
	}

	lb.mu.Lock()
	if len(lb.logs) == 0 && len(lb.failedLogs) == 0 {
		lb.mu.Unlock()
		return
	}

	// 실패 로그 복사
	var retryLogs []retryableLog
	if len(lb.failedLogs) > 0 {
		retryLogs = make([]retryableLog, len(lb.failedLogs))
		copy(retryLogs, lb.failedLogs)
		lb.failedLogs = lb.failedLogs[:0]
	}

	// 새 로그 복사
	var newLogs []models.AGVLog
	if len(lb.logs) > 0 {
		newLogs = make([]models.AGVLog, len(lb.logs))
		copy(newLogs, lb.logs)
		lb.logs = lb.logs[:0]
	}
	lb.mu.Unlock()

	// 1) 실패 로그 재시도
	var stillFailed []retryableLog
	if len(retryLogs) > 0 {
		retryBatch := make([]models.AGVLog, len(retryLogs))
		for i, rl := range retryLogs {
			retryBatch[i] = rl.Log
		}
		if err := db.CreateInBatches(retryBatch, 100).Error; err != nil {
			log.Printf("❌ 재시도 로그 저장 실패: %v", err)
			for _, rl := range retryLogs {
				rl.RetryCount++
				if rl.RetryCount >= maxRetries {
					log.Printf("❌ 로그 재시도 %d회 초과, 폐기", maxRetries)
				} else {
					stillFailed = append(stillFailed, rl)
				}
			}
		} else {
			log.Printf("💾 재시도 로그 %d개 저장 완료", len(retryBatch))
		}
	}

	// 2) 새 로그 저장
	if len(newLogs) > 0 {
		if err := db.CreateInBatches(newLogs, 100).Error; err != nil {
			log.Printf("❌ 로그 저장 실패: %v", err)
			for _, l := range newLogs {
				stillFailed = append(stillFailed, retryableLog{Log: l, RetryCount: 0})
			}
		} else {
			log.Printf("💾 로그 %d개 저장 완료", len(newLogs))
		}
	}

	// 3) 실패 로그를 큐에 복원 (최대 크기 제한)
	if len(stillFailed) > 0 {
		lb.mu.Lock()
		lb.failedLogs = append(lb.failedLogs, stillFailed...)
		if len(lb.failedLogs) > maxFailedLogs {
			dropped := len(lb.failedLogs) - maxFailedLogs
			lb.failedLogs = lb.failedLogs[dropped:] // 앞쪽(오래된 것)부터 버림
			log.Printf("⚠️ 실패 로그 큐 초과, %d개 폐기", dropped)
		}
		lb.mu.Unlock()
	}
}

// 🆕 LogAGVPosition - AGV 위치 로그
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

// 🆕 LogAGVStatus - AGV 상태 로그
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

	// 타겟 정보
	if status.TargetEnemy != nil {
		logEntry.TargetEnemyID = status.TargetEnemy.ID
		logEntry.TargetEnemyHP = status.TargetEnemy.HP
		logEntry.TargetEnemyName = status.TargetEnemy.Name
	}

	AddLog(logEntry)
}

// 🆕 LogTargetFound - 적 발견 로그
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

// 🆕 LogCommand - 명령 로그
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

// 🆕 LogAIExplanation - AI 설명 로그
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

// 🆕 LogWebSocketMessage - WebSocket 메시지 로그 (범용)
func LogWebSocketMessage(agvID string, msg models.WebSocketMessage) {
	dataJSON, _ := json.Marshal(msg.Data)

	logEntry := models.AGVLog{
		CreatedAt:   time.Now(),
		EventType:   inferEventType(msg.Type),
		MessageType: msg.Type,
		AGVID:       agvID,
		DataJSON:    string(dataJSON),
	}

	// 타입별 추가 데이터 추출
	extractLogData(&logEntry, msg)

	AddLog(logEntry)
}

// extractLogData - 메시지에서 데이터 추출
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

// inferEventType - 메시지 타입에서 이벤트 타입 추론
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

// 🆕 GetLogsByTimeRange - 시간 범위로 로그 조회
func GetLogsByTimeRange(agvID string, start, end time.Time, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	query := db.Where("agv_id = ? AND created_at BETWEEN ? AND ?", agvID, start, end)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

// 🆕 GetLogsByEventType - 이벤트 타입별 로그 조회
func GetLogsByEventType(agvID string, eventType string, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	err := db.Where("agv_id = ? AND event_type = ?", agvID, eventType).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// 🆕 GetLogStats - 로그 통계
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

// StopLogging - 로깅 시스템 종료
func StopLogging() {
	if logBuffer != nil {
		logBuffer.stopChan <- true
		log.Println("🛑 로깅 시스템 종료")
	}
}
