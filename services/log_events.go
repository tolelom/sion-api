package services

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"time"
)

// LogAGVEvent는 외부 호출(WebSocket 핸들러 등)에서 들어오는 메시지를 일관되게
// LogWebSocketMessage로 흘려보내기 위한 진입점이다. AGV/웹 양쪽 모두 동일 경로를 탄다.
func LogAGVEvent(msg models.WebSocketMessage, agvID string, userID string) error {
	_ = userID // 향후 user 단위 audit를 위해 보존
	LogWebSocketMessage(agvID, msg)
	return nil
}

func LogAGVPosition(agvID string, position models.PositionData) {
	AddLog(models.AGVLog{
		CreatedAt:     time.Now(),
		EventType:     "position_update",
		MessageType:   "position",
		AGVID:         agvID,
		PositionX:     position.X,
		PositionY:     position.Y,
		PositionAngle: position.Angle,
	})
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
	AddLog(models.AGVLog{
		CreatedAt:       time.Now(),
		EventType:       "target_found",
		MessageType:     "target_found",
		AGVID:           agvID,
		TargetEnemyID:   target.ID,
		TargetEnemyHP:   target.HP,
		TargetEnemyName: target.Name,
	})
}

func LogCommand(agvID string, commandType string, targetX, targetY float64) {
	AddLog(models.AGVLog{
		CreatedAt:   time.Now(),
		EventType:   "command",
		MessageType: "command",
		AGVID:       agvID,
		CommandType: commandType,
		TargetX:     targetX,
		TargetY:     targetY,
	})
}

func LogAIExplanation(agvID string, eventType string, explanation string) {
	AddLog(models.AGVLog{
		CreatedAt:     time.Now(),
		EventType:     eventType,
		MessageType:   "agv_event",
		AGVID:         agvID,
		AIExplanation: explanation,
	})
}

func LogWebSocketMessage(agvID string, msg models.WebSocketMessage) {
	dataJSON, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("[WARN] 로그 데이터 직렬화 실패 (type=%s): %v", msg.Type, err)
		dataJSON = nil
	}

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

	if x, ok := dataMap["x"].(float64); ok {
		logEntry.PositionX = x
	}
	if y, ok := dataMap["y"].(float64); ok {
		logEntry.PositionY = y
	}
	if angle, ok := dataMap["angle"].(float64); ok {
		logEntry.PositionAngle = angle
	}

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
