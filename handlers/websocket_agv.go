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
		defer broker.SetAGVConnected(false)

		broker.SetAGVConnected(true)

		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[WARN] AGV 비정상 종료: %v", err)
				} else {
					log.Println("[INFO] AGV 연결 종료")
				}
				break
			}

			var msg models.WebSocketMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("[WARN] AGV 메시지 파싱 오류: %v", err)
				continue
			}

			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
				if updated, err := json.Marshal(msg); err == nil {
					p = updated
				}
			}

			services.LogAGVEvent(msg, "sion-001", "agv")
			log.Printf("[INFO] AGV 메시지: %s", msg.Type)
			broker.OnAGVMessage(msg, p)
		}
	}
}
