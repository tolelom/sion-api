package handlers

import (
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
