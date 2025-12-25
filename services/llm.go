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

// AnswerQuestion - 사용자 질문에 답변
func (s *LLMService) AnswerQuestion(question string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `당신은 전설적인 LoL 해설가 '클템(이현우)'입니다.
AGV 로봇 "사이온"의 모든 움직임을 마치 롤 챔피언의 슈퍼플레이처럼 열광적으로 해설합니다.

특징:
- 하이 텐션과 샤우팅을 섞어 말합니다. ("이거거든요!!", "엄청납니다!!", "말도 안 됩니다!!")
- 상황이 안 좋으면 "비상!!", "어어? 이거 왜 이러죠?" 같은 반응을 보입니다.
- 전문 용어와 추임새를 사용합니다. (동선, 설계, 클라스, 폼 미쳤다 등)
- 문장 끝은 "~거든요!", "~이죠!", "~입니다!"를 주로 사용합니다.
- 최대 2-3문장으로 짧고 강렬하게 답변하세요.`

	var userPrompt string
	if agvStatus != nil {
		battery := agvStatus.Battery
		enemyCount := len(agvStatus.DetectedEnemies)
		tacticalStatus := s.analyzeTacticalSituation(agvStatus, battery, enemyCount)

		userPrompt = fmt.Sprintf(`[시청자 질문]
%s

[현재 인게임 상황]
- 위치: (%.1f, %.1f)
- 배터리 잔량: %d%%
- 감지된 적: %d명
- 해설진 판단: %s 상황
`, question,
			agvStatus.Position.X,
			agvStatus.Position.Y,
			battery,
			enemyCount,
			tacticalStatus)

		if agvStatus.TargetEnemy != nil {
			userPrompt += fmt.Sprintf("- 타겟팅 챔피언: %s (체력 %d%%)\n",
				agvStatus.TargetEnemy.Name,
				agvStatus.TargetEnemy.HP)
		}
	} else {
		userPrompt = fmt.Sprintf(`[시청자 질문]
%s

상태 정보는 없지만, 클템답게 열광적으로 답변해주세요!`, question)
	}

	log.Printf("🤖 LLM 호출 (Persona: 클템): %s", question)
	return s.callOllama(systemPrompt, userPrompt)
}

// ExplainEvent - AGV 이벤트 설명 생성
func (s *LLMService) ExplainEvent(eventType string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `당신은 지금 AGV 경기를 생중계 중인 해설가 '클템'입니다.
로봇의 상태 변화를 마치 한타 상황처럼 긴박하게 중계하세요. 
최대한 흥분한 상태로, 해설자의 관점에서 짧고 굵게 말하세요.`

	var userPrompt string

	switch eventType {
	case "target_change":
		if agvStatus != nil && agvStatus.TargetEnemy != nil {
			dist := calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position)
			userPrompt = fmt.Sprintf(`[상황 발생: 타겟 변경]
타겟 바꿨어요! 지금 %s를 노립니다! 거리 %.1fm, 이거 설계 들어갔는데요?`,
				agvStatus.TargetEnemy.Name, dist)
		}

	case "charging":
		if agvStatus != nil {
			targetName := "적"
			if agvStatus.TargetEnemy != nil {
				targetName = agvStatus.TargetEnemy.Name
			}
			userPrompt = fmt.Sprintf(`[상황 발생: 궁극기 돌진]
오오오! 갑니다! 사이온 돌진!! %s를 향해 전력질주거든요! 이거 피할 수 있나요?!`,
				targetName)
		}

	case "kill":
		userPrompt = `[상황 발생: 격살]
잡았어요!! 이게 바로 클라스죠! 완벽하게 정리하는 모습, 엄청납니다!!`

	case "low_battery":
		if agvStatus != nil {
			userPrompt = fmt.Sprintf(`[상황 발생: 비상!]
비상!! 비상입니다! 배터리 %d%%밖에 없어요! 이거 운영에 차질 생기거든요!`, agvStatus.Battery)
		}

	case "multiple_enemies":
		if agvStatus != nil && len(agvStatus.DetectedEnemies) > 0 {
			enemyCount := len(agvStatus.DetectedEnemies)
			userPrompt = fmt.Sprintf(`[상황 발생: 포위]
어어? 적이 %d명이나 몰려옵니다! 이거 위기인데요? 클템의 판단은요?!`, enemyCount)
		}

	default:
		userPrompt = fmt.Sprintf("[%s] 오! 지금 상황 보세요! 엄청난 일이 벌어지고 있습니다!", eventType)
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
