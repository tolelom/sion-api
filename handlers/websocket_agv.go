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
		defer broker.SetAGVConnected(false) // LIFO: 브로드캐스트 후 연결 해제

		broker.SetAGVConnected(true)

		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("WARN: AGV 비정상 종료: %v", err)
				} else {
					log.Printf("INFO: AGV 연결 종료")
				}
				break
			}

			var msg models.WebSocketMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("WARN: AGV 메시지 파싱 오류 (연결 유지): %v", err)
				continue // AGV에는 에러 응답 전송하지 않음
			}

			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
				// timestamp 주입 시 raw bytes 갱신 (웹 클라이언트에 정확한 timestamp 전달)
				if updated, err := json.Marshal(msg); err == nil {
					p = updated
				}
			}
			if msg.Type == models.MessageTypePathUpdate {
				logPathUpdate(msg)
			}
			services.LogAGVEvent(msg, "sion-001", "agv")
			log.Printf("AGV 메시지: %s", msg.Type)
			broker.OnAGVMessage(msg, p)
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
