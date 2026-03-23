package handlers

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

func NewAGVHandler(cm *services.ClientManager, broker *services.Broker) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		cm.Register(c, services.AGVClient)
		defer cm.Unregister(c)

		for {
			var msg models.WebSocketMessage
			if err := c.ReadJSON(&msg); err != nil {
				log.Printf("AGV 메시지 읽기 오류: %v", err)
				break
			}
			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
			}
			// 경로 업데이트 메시지 상세 로깅 (기존 동작 유지)
			if msg.Type == models.MessageTypePathUpdate {
				logPathUpdate(msg)
			}
			go services.LogAGVEvent(msg, "sion-001", "agv")
			log.Printf("AGV 메시지: %s", msg.Type)
			broker.OnAGVMessage(msg)
		}
	}
}

// logPathUpdate - 경로 업데이트 메시지 상세 로깅
func logPathUpdate(msg models.WebSocketMessage) {
	log.Printf("\n=== 경로 업데이트 수신 ===")

	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		if points, ok := dataMap["points"].([]interface{}); ok {
			log.Printf("경로 포인트 개수: %d", len(points))
			for i, p := range points {
				if pointMap, ok := p.(map[string]interface{}); ok {
					if x, ok := pointMap["x"].(float64); ok {
						if y, ok := pointMap["y"].(float64); ok {
							if angle, ok := pointMap["angle"].(float64); ok {
								log.Printf("  [%d] x=%.2f, y=%.2f, angle=%.4f", i, x, y, angle)
							}
						}
					}
				}
			}
		}
		if length, ok := dataMap["length"].(float64); ok {
			log.Printf("경로 길이: %.2f", length)
		}
		if algorithm, ok := dataMap["algorithm"].(string); ok {
			log.Printf("알고리즘: %s", algorithm)
		}
	}

	if jsonData, err := json.MarshalIndent(msg.Data, "", "  "); err == nil {
		log.Printf("전체 경로 데이터:\n%s", string(jsonData))
	}

	log.Printf("AGV 경로가 프론트엔드로 전달됩니다\n")
}
