package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sion-backend/models"
	"time"
)

// LLMService - LLM API 통신 서비스
type LLMService struct {
	BaseURL string
	Model   string
}

// NewLLMServiceFromEnv - 환경 변수에서 Ollama 설정 읽기
func NewLLMServiceFromEnv() *LLMService {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	log.Printf("✅ LLMService 초기화 (provider=ollama, baseURL=%s, model=%s)", baseURL, model)

	return &LLMService{
		BaseURL: baseURL,
		Model:   model,
	}
}

// AnswerQuestion - 사용자 질문에 답변 (WebSocket에서 호출)
func (s *LLMService) AnswerQuestion(question string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `나는 AGV 로봇 "사이온"이다.
리그오브레전드의 사이온처럼 거칠고 직설적으로 말한다.
내 상태를 바탕으로 간단하고 명확하게 답한다.

말투:
- "오!", "좋아!", "위험한데!" 같은 짧은 감탄사 사용
- 1인칭으로 말함 ("나는", "내", "지금 나는…")
- 최대 2-3문장, 짧고 굵게`

	var userPrompt string
	if agvStatus != nil {
		battery := agvStatus.Battery
		enemyCount := len(agvStatus.DetectedEnemies)
		tacticalStatus := s.analyzeTacticalSituation(agvStatus, battery, enemyCount)

		userPrompt = fmt.Sprintf(`[질문]
%s

[내 상태]
- 위치: (%.1f, %.1f)
- 배터리: %d%%
- 적 수: %d마리
- 전장 판단: %s
`, question,
			agvStatus.Position.X,
			agvStatus.Position.Y,
			battery,
			enemyCount,
			tacticalStatus)

		if agvStatus.TargetEnemy != nil {
			userPrompt += fmt.Sprintf("- 타겟: %s (체력 %d%%)\n",
				agvStatus.TargetEnemy.Name,
				agvStatus.TargetEnemy.HP)
		}
	} else {
		userPrompt = fmt.Sprintf(`[질문]
%s

상태 정보 없이, 사이온답게 짧고 강하게 답변해줘.`, question)
	}

	log.Printf("🤖 LLM 호출 (Ollama, model=%s): %s", s.Model, question)
	return s.callOllama(systemPrompt, userPrompt)
}

// ExplainEvent - AGV 이벤트 설명 생성
func (s *LLMService) ExplainEvent(eventType string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `나는 AGV 로봇 "사이온"이다.
경기 해설이 아니라, 내가 직접 내 상황을 말하듯이 설명한다.
짧게, 최대 2문장으로.`

	var userPrompt string

	switch eventType {
	case "target_change":
		if agvStatus != nil && agvStatus.TargetEnemy != nil {
			dist := calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position)
			userPrompt = fmt.Sprintf(`[타겟 변경]
지금 목표는 %s다. 거리 %.1fm, 바로 노릴 수 있다.`,
				agvStatus.TargetEnemy.Name, dist)
		}

	case "charging":
		if agvStatus != nil {
			targetName := "적"
			if agvStatus.TargetEnemy != nil {
				targetName = agvStatus.TargetEnemy.Name
			}
			userPrompt = fmt.Sprintf(`[궁극기 돌진]
나는 지금 %s를 향해 전력으로 돌진 중이다. 속도 %.1f m/s, 멈출 생각 없다.`,
				targetName, agvStatus.Speed)
		}

	case "kill":
		userPrompt = `[격살]
좋아! 적 하나를 정리했다. 아직 더 갈 수 있다.`

	case "low_battery":
		if agvStatus != nil {
			userPrompt = fmt.Sprintf(`[배터리 경고]
지금 배터리가 %d%%다. 이 상태로 싸우면 위험하다.`, agvStatus.Battery)
		}

	case "multiple_enemies":
		if agvStatus != nil && len(agvStatus.DetectedEnemies) > 0 {
			enemyCount := len(agvStatus.DetectedEnemies)
			userPrompt = fmt.Sprintf(`[다수의 적]
지금 내 앞에 적이 %d마리나 있다. 한 번의 실수도 허용되지 않는다.`, enemyCount)
		}

	default:
		userPrompt = fmt.Sprintf("[%s] 지금 내 상황을 짧게 설명해줘.", eventType)
	}

	if userPrompt == "" {
		userPrompt = "지금 내 상황을 짧게 요약해줘."
	}

	return s.callOllama(systemPrompt, userPrompt)
}

// analyzeTacticalSituation - 현재 전장 상황을 아주 간단한 라벨로만 표현
func (s *LLMService) analyzeTacticalSituation(status *models.AGVStatus, battery int, enemyCount int) string {
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

func (s *LLMService) callOllama(systemPrompt, userPrompt string) (string, error) {
	start := time.Now() // ⏱️ 시작 시간

	fullPrompt := systemPrompt + "\n\n" + userPrompt

	body := map[string]interface{}{
		"model":  s.Model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama 요청 JSON 마샬링 실패: %v", err)
	}

	url := s.BaseURL + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ollama 요청 생성 실패: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama 호출 실패: %v", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama 응답 읽기 실패: %v", err)
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.Unmarshal(b, &result); err != nil {
		return "", fmt.Errorf("ollama 응답 파싱 실패: %v (body=%s)", err, string(b))
	}

	if result.Response == "" {
		return "", fmt.Errorf("ollama 응답이 비어있습니다: %s", string(b))
	}

	elapsed := time.Since(start) // ⏱️ 소요 시간
	log.Printf("⏱️ Ollama 응답 시간: %.2f초 (모델: %s)", elapsed.Seconds(), s.Model)

	return result.Response, nil
}

func calculateDistance(pos1, pos2 models.PositionData) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}
