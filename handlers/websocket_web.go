package handlers

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

func NewWebHandler(cm *services.ClientManager, sim *services.AGVSimulator) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		cm.Register(c)
		defer cm.Unregister(c)
		stopKeepalive := installKeepalive(cm, c, "Web")
		defer stopKeepalive()

		sendWelcome(cm, c, sim)

		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[WARN] 웹 비정상 종료: %v", err)
				}
				return
			}

			var msg models.WebSocketMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				sendError(cm, c, "잘못된 메시지 형식")
				continue
			}
			if err := sim.HandleCommand(msg); err != nil {
				sendError(cm, c, err.Error())
			}
		}
	}
}

func sendWelcome(cm *services.ClientManager, c *websocket.Conn, sim *services.AGVSimulator) {
	now := time.Now()
	welcome := models.WebSocketMessage{
		Type: models.MessageTypeSystemInfo,
		Data: map[string]interface{}{
			"agv_connected": true,
			"server_time":   now.Format(time.RFC3339),
		},
		Timestamp: now.UnixMilli(),
	}
	for _, msg := range []models.WebSocketMessage{welcome, sim.MapUpdateMessage()} {
		if err := cm.WriteJSON(c, msg); err != nil {
			log.Printf("[WARN] 초기 메시지 전송 실패: %v", err)
			return
		}
	}
}

func sendError(cm *services.ClientManager, c *websocket.Conn, reason string) {
	errMsg := models.WebSocketMessage{
		Type:      models.MessageTypeError,
		Data:      map[string]interface{}{"message": reason},
		Timestamp: time.Now().UnixMilli(),
	}
	if err := cm.WriteJSON(c, errMsg); err != nil {
		log.Printf("[WARN] 에러 메시지 전송 실패: %v", err)
	}
}
