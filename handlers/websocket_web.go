package handlers

import (
	"encoding/json"
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

		welcomeData := map[string]interface{}{
			"message":       "웹 클라이언트 연결됨",
			"connected_at":  time.Now().Format(time.RFC3339),
			"agv_connected": broker.IsAGVConnected(),
		}
		if agvStatus := broker.GetAGVStatus(); agvStatus != nil {
			welcomeData["agv_status"] = agvStatus
		}
		welcomeMsg := models.WebSocketMessage{
			Type:      models.MessageTypeSystemInfo,
			Data:      welcomeData,
			Timestamp: time.Now().UnixMilli(),
		}
		_ = c.WriteJSON(welcomeMsg)

		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[WARN] 웹 비정상 종료: %v", err)
				} else {
					log.Println("[INFO] 웹 연결 종료")
				}
				break
			}

			var msg models.WebSocketMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("[WARN] 웹 메시지 파싱 오류: %v", err)
				errMsg := models.WebSocketMessage{
					Type:      models.MessageTypeError,
					Data:      map[string]interface{}{"message": "invalid message format"},
					Timestamp: time.Now().UnixMilli(),
				}
				_ = c.WriteJSON(errMsg)
				continue
			}

			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
			}
			go services.LogAGVEvent(msg, "sion-001", "web-user")
			log.Printf("[INFO] 웹 메시지: %s", msg.Type)

			switch msg.Type {
			case models.MessageTypeChat:
				var chatData models.ChatMessageData
				raw, err := json.Marshal(msg.Data)
				if err != nil {
					log.Printf("[WARN] chat marshal 실패: %v", err)
					break
				}
				if err := json.Unmarshal(raw, &chatData); err != nil {
					log.Printf("[WARN] chat 파싱 실패: %v", err)
					break
				}
				if chatData.Message != "" {
					go handleChatViaWebSocket(chatData.Message, broker, llm)
				}
			case models.MessageTypeCommand,
				models.MessageTypeModeChange,
				models.MessageTypeEmergencyStop:
				broker.OnWebMessage(msg)
			default:
				log.Printf("[WARN] 알 수 없는 메시지 타입: %s", msg.Type)
			}
		}
	}
}

func handleChatViaWebSocket(message string, broker *services.Broker, llm *services.LLMService) {
	if llm == nil {
		log.Println("[WARN] LLM 서비스 미초기화")
		return
	}
	status := broker.GetAGVStatus()
	response, err := llm.AnswerQuestion(message, status)
	if err != nil {
		log.Printf("[ERROR] LLM 응답 실패: %v", err)
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
