package handlers

import (
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ChatHandler는 LLM/Broker 의존성을 명시적으로 보유한다.
// 이전 구현은 패키지 전역(llmService, broker) + Init*() 패턴이라
// 초기화 누락 시 nil-deref 위험과 테스트 격리 어려움이 있었다.
type ChatHandler struct {
	llm    *services.LLMService
	broker *services.Broker
}

func NewChatHandler(llm *services.LLMService, broker *services.Broker) *ChatHandler {
	return &ChatHandler{llm: llm, broker: broker}
}

func (h *ChatHandler) HandleChat(c *fiber.Ctx) error {
	var chatData models.ChatMessageData
	if err := c.BodyParser(&chatData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "잘못된 요청 형식",
		})
	}

	if h.llm == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "LLM 서비스가 초기화되지 않았습니다",
		})
	}

	log.Printf("[INFO] 채팅 수신: %s", chatData.Message)

	var status *models.AGVStatus
	if h.broker != nil {
		if s, ok := h.broker.GetAGVStatus(); ok {
			status = &s
		}
	}

	response, err := h.llm.AnswerQuestion(chatData.Message, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "AI 응답 생성 실패: " + err.Error(),
		})
	}

	if h.broker != nil {
		h.broker.BroadcastToWeb(models.NewMessage(models.MessageTypeChatResponse, models.ChatResponseData{
			Message:   response,
			Model:     h.llm.Model,
			Timestamp: time.Now().UnixMilli(),
		}, time.Now().UnixMilli()))
	}

	return c.JSON(fiber.Map{"success": true, "response": response})
}

// ExplainAGVEvent는 LLM으로 이벤트 해설을 생성해 web으로 브로드캐스트한다.
// LLM 호출은 외부 IO라 호출자를 블록하지 않도록 항상 비동기로 수행한다.
func (h *ChatHandler) ExplainAGVEvent(eventType string, agvStatus *models.AGVStatus) {
	if h.llm == nil {
		return
	}
	go func() {
		explanation, err := h.llm.ExplainEvent(eventType, agvStatus)
		if err != nil {
			log.Printf("[ERROR] 이벤트 설명 생성 실패: %v", err)
			return
		}
		if h.broker == nil {
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
		h.broker.BroadcastToWeb(models.NewMessage(models.MessageTypeAGVEvent, eventData, time.Now().UnixMilli()))
	}()
}
