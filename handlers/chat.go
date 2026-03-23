package handlers

import (
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

var llmService *services.LLMService
var broker *services.Broker

// InitLLMService - LLM 서비스 초기화
func InitLLMService() {
	llmService = services.NewLLMServiceFromEnv()
	if llmService == nil {
		log.Println("LLM 서비스 초기화 실패")
		return
	}
	log.Printf("LLM 서비스 초기화 완료 (Ollama, model=%s)", llmService.Model)
}

// InitBroker - Broker 주입 (main.go에서 호출)
func InitBroker(b *services.Broker) {
	broker = b
}

// GetLLMService - LLM 서비스 인스턴스 반환 (main.go에서 핸들러 생성 시 사용)
func GetLLMService() *services.LLMService {
	return llmService
}

// HandleChat - 채팅 메시지 처리 (HTTP POST)
func HandleChat(c *fiber.Ctx) error {
	var chatData models.ChatMessageData
	if err := c.BodyParser(&chatData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "잘못된 요청 형식",
		})
	}

	if llmService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "LLM 서비스가 초기화되지 않았습니다",
		})
	}

	log.Printf("채팅 수신: %s", chatData.Message)

	var status *models.AGVStatus
	if broker != nil {
		status = broker.GetAGVStatus()
	}

	response, err := llmService.AnswerQuestion(chatData.Message, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "AI 응답 생성 실패: " + err.Error(),
		})
	}

	if broker != nil {
		broker.BroadcastToWeb(models.WebSocketMessage{
			Type: models.MessageTypeChatResponse,
			Data: models.ChatResponseData{
				Message:   response,
				Model:     llmService.Model,
				Timestamp: time.Now().UnixMilli(),
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}

	return c.JSON(fiber.Map{"success": true, "response": response})
}

// ExplainAGVEvent - AGV 이벤트 자동 설명 (내부 호출용)
func ExplainAGVEvent(eventType string, agvStatus *models.AGVStatus) {
	if llmService == nil {
		return
	}
	go func() {
		explanation, err := llmService.ExplainEvent(eventType, agvStatus)
		if err != nil {
			log.Printf("이벤트 설명 생성 실패: %v", err)
			return
		}
		if broker != nil {
			broker.BroadcastToWeb(models.WebSocketMessage{
				Type: models.MessageTypeAGVEvent,
				Data: models.AGVEventData{
					EventType:   eventType,
					Explanation: explanation,
					Position:    agvStatus.Position,
					Timestamp:   time.Now().UnixMilli(),
				},
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}()
}
