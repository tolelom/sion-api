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

func InitLLMService() {
	llmService = services.NewLLMServiceFromEnv()
	if llmService == nil {
		log.Println("[WARN] LLM 서비스 초기화 실패")
		return
	}
	log.Printf("[INFO] LLM 서비스 초기화 완료 (model=%s)", llmService.Model)
}

func InitBroker(b *services.Broker) {
	broker = b
}

func GetLLMService() *services.LLMService {
	return llmService
}

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

	log.Printf("[INFO] 채팅 수신: %s", chatData.Message)

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

func ExplainAGVEvent(eventType string, agvStatus *models.AGVStatus) {
	if llmService == nil {
		return
	}
	go func() {
		explanation, err := llmService.ExplainEvent(eventType, agvStatus)
		if err != nil {
			log.Printf("[ERROR] 이벤트 설명 생성 실패: %v", err)
			return
		}
		if broker == nil {
			return
		}
		eventData := models.AGVEventData{
			EventType:   eventType,
			Explanation: explanation,
			Timestamp:   time.Now().UnixMilli(),
		}
		if agvStatus != nil {
			eventData.Position = agvStatus.Position
		}
		broker.BroadcastToWeb(models.WebSocketMessage{
			Type:      models.MessageTypeAGVEvent,
			Data:      eventData,
			Timestamp: time.Now().UnixMilli(),
		})
	}()
}
