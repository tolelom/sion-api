package handlers

import (
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

func NewWebHandler(cm *services.ClientManager, broker *services.Broker, llm *services.LLMService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		cm.Register(c, services.WebClient)
		defer cm.Unregister(c)

		// 환영 메시지
		welcomeMsg := models.WebSocketMessage{
			Type: models.MessageTypeSystemInfo,
			Data: map[string]interface{}{
				"message":      "웹 클라이언트 연결됨",
				"connected_at": time.Now().Format(time.RFC3339),
			},
			Timestamp: time.Now().UnixMilli(),
		}
		_ = c.WriteJSON(welcomeMsg)

		for {
			var msg models.WebSocketMessage
			if err := c.ReadJSON(&msg); err != nil {
				log.Printf("웹 메시지 읽기 오류: %v", err)
				break
			}
			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
			}
			go services.LogAGVEvent(msg, "sion-001", "web-user")
			log.Printf("웹 메시지: %s", msg.Type)

			switch msg.Type {
			case models.MessageTypeChat:
				if chatData, ok := msg.Data.(map[string]interface{}); ok {
					if message, ok := chatData["message"].(string); ok {
						go handleChatViaWebSocket(message, broker, llm)
					}
				}
			case models.MessageTypeCommand,
				models.MessageTypeModeChange,
				models.MessageTypeEmergencyStop:
				broker.OnWebMessage(msg)
			default:
				log.Printf("알 수 없는 메시지 타입: %s", msg.Type)
			}
		}
	}
}

func handleChatViaWebSocket(message string, broker *services.Broker, llm *services.LLMService) {
	if llm == nil {
		log.Println("LLM 서비스 미초기화")
		return
	}
	status := broker.GetAGVStatus()
	response, err := llm.AnswerQuestion(message, status)
	if err != nil {
		log.Printf("LLM 응답 실패: %v", err)
		return
	}
	broker.BroadcastToWeb(models.WebSocketMessage{
		Type: models.MessageTypeChatResponse,
		Data: models.ChatResponseData{
			Message:   response,
			Model:     llm.Model,
			Timestamp: time.Now().UnixMilli(),
		},
		Timestamp: time.Now().UnixMilli(),
	})
}
