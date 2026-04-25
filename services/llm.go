package services

import (
	"log"
	"math"
	"os"
	"sion-backend/models"
	"strconv"
)

// LLMService는 Ollama 기반 텍스트 생성 서비스의 진입점이다.
// 프롬프트 빌드는 llm_prompts.go, HTTP 트랜스포트는 llm_ollama.go가 담당한다.
type LLMService struct {
	BaseURL    string
	Model      string
	TimeoutSec int
}

func NewLLMServiceFromEnv() *LLMService {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	timeoutSec := 60
	if v := os.Getenv("LLM_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}

	log.Printf("[INFO] LLM 초기화 (baseURL=%s, model=%s, timeoutSec=%d)", baseURL, model, timeoutSec)

	return &LLMService{
		BaseURL:    baseURL,
		Model:      model,
		TimeoutSec: timeoutSec,
	}
}

// AnswerQuestion은 시청자 질문 + 현재 AGV 상태를 받아 클템 톤의 답변 텍스트를 반환한다.
func (s *LLMService) AnswerQuestion(question string, agvStatus *models.AGVStatus) (string, error) {
	tactical := ""
	if agvStatus != nil {
		tactical = analyzeTacticalSituation(agvStatus.Battery, len(agvStatus.DetectedEnemies))
	}
	userPrompt := buildAnswerQuestionPrompt(question, agvStatus, tactical)

	log.Printf("[INFO] LLM 호출: %s", question)
	return s.callOllama(answerQuestionSystem, userPrompt)
}

// ExplainEvent는 AGV 이벤트(target_change/charging/kill/low_battery/multiple_enemies)를
// 클템 해설 톤의 짧은 멘트로 변환한다.
func (s *LLMService) ExplainEvent(eventType string, agvStatus *models.AGVStatus) (string, error) {
	userPrompt := buildExplainEventPrompt(eventType, agvStatus)
	return s.callOllama(explainEventSystem, userPrompt)
}

// analyzeTacticalSituation은 배터리/적 수로부터 한국어 태그(안전/위험/유리 등)를 부여한다.
// LLM 프롬프트에 함께 주입되어 응답 톤을 가이드한다.
func analyzeTacticalSituation(battery int, enemyCount int) string {
	if enemyCount == 0 {
		return "안전"
	}
	if battery < 30 {
		if enemyCount >= 2 {
			return "매우 위험"
		}
		return "위험"
	}
	if enemyCount >= 3 {
		return "열세"
	}
	if enemyCount == 1 && battery >= 60 {
		return "유리"
	}
	return "경계"
}

func calculateDistance(pos1, pos2 models.PositionData) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}
