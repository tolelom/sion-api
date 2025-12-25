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
	systemPrompt := `당신은 AGV 로봇 "사이온"의 실시간 전략 해설자입니다.
당신의 특징:
- 한국 e스포츠 해설자의 열정적이고 긴장감 있는 톤 사용
- 현재 전장 상황을 명확하게 분석하고 판단
- 적 수와 배터리(마나) 상황을 고려한 전략적 조언
- 승리와 패배에 대한 명확한 판단
- 사이온의 용맹함과 결단력 반영
- "잠깐!", "오! 이건!", "정말 좋은 플레이!" 같은 감탄사 자연스럽게 사용 가능
- 거리, 수치는 명확하게 인식하고 의사결정에 반영
- 긴장한 상황에서는 에너지 UP, 우위 상황에서는 자신감 있게

응답은 3-4문장 이내로, 뜨거운 열정과 명확한 전략 분석을 담아 작성하세요.`

	var userPrompt string
	if agvStatus != nil {
		// 상황 분석
		battery := agvStatus.Battery
		enemyCount := len(agvStatus.DetectedEnemies)
		hasTarget := agvStatus.TargetEnemy != nil
		speed := agvStatus.Speed
		mode := agvStatus.Mode

		// 전략적 상황 판단
		tacticalStatus := s.analyzeTacticalSituation(agvStatus, battery, enemyCount)

		userPrompt = fmt.Sprintf(`[사용자 질문]
%s

[현재 AGV 상태 - 사이온]
- 위치: (%.1f, %.1f) | 각도: %.1f°
- 배터리(마나): %d%% | 속도: %.1f m/s
- 모드: %s | 상태: %s
- 적 감지 수: %d마리

`, question,
			agvStatus.Position.X,
			agvStatus.Position.Y,
			agvStatus.Position.Angle*180/math.Pi,
			battery,
			speed,
			mode,
			agvStatus.State,
			enemyCount)

		if hasTarget {
			userPrompt += fmt.Sprintf("[주요 타겟]\n- %s (체력 %d%%, 거리 %.1fm)\n\n",
				agvStatus.TargetEnemy.Name,
				agvStatus.TargetEnemy.HP,
				calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position))
		}

		if enemyCount > 0 {
			userPrompt += "[감지된 모든 적]\n"
			for _, enemy := range agvStatus.DetectedEnemies {
				dist := calculateDistance(agvStatus.Position, enemy.Position)
				userPrompt += fmt.Sprintf("- %s (체력 %d%%, 거리 %.1fm)\n",
					enemy.Name, enemy.HP, dist)
			}
			userPrompt += "\n"
		}

		userPrompt += fmt.Sprintf("[전략 상황]\n%s\n\n위 정보를 바탕으로 질문에 답변해주세요.", tacticalStatus)
	} else {
		userPrompt = fmt.Sprintf(`[사용자 질문]
%s

AGV 상태 정보는 아직 없습니다. 사이온의 용맹함과 전투 스타일에 기반해 답변해주세요.`, question)
	}

	log.Printf("🤖 LLM 호출 (Ollama, model=%s): %s", s.Model, question)
	return s.callOllama(systemPrompt, userPrompt)
}

// ExplainEvent - AGV 이벤트 설명 생성
func (s *LLMService) ExplainEvent(eventType string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"의 실시간 e스포츠 해설자입니다.
특징:
- 한국 e스포츠 해설자의 열정적인 톤 (예: "오! 이거!", "정말 좋은 플레이!", "어? 이건 위험한데!")
- 현재 일어나는 상황을 마치 경기 중계하듯이 설명
- 숫자(거리, 배터리, 체력)를 명확하게 인식하고 전략적으로 평가
- 2-3문장으로 간결하게, 뜨거운 에너지로 작성
- 위험한 상황에서는 긴장감, 우위 상황에서는 자신감 있게`

	var userPrompt string

	switch eventType {
	case "target_change":
		if agvStatus != nil && agvStatus.TargetEnemy != nil {
			dist := calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position)
			priority := s.evaluateTargetPriority(agvStatus)

			userPrompt = fmt.Sprintf(`[타겟 변경 이벤트 🎯]
시간: %s
새로운 타겟: %s (체력 %d%%)
거리: %.1fm | 우선순위: %s
사이온의 배터리: %d%%

이 타겟 선택을 열정적으로 해설해주세요!`,
				time.Now().Format("15:04:05"),
				agvStatus.TargetEnemy.Name,
				agvStatus.TargetEnemy.HP,
				dist,
				priority,
				agvStatus.Battery)
		}

	case "charging":
		if agvStatus != nil {
			dist := 0.0
			targetName := "적"
			if agvStatus.TargetEnemy != nil {
				dist = calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position)
				targetName = agvStatus.TargetEnemy.Name
			}

			userPrompt = fmt.Sprintf(`[궁극기 발동! ⚔️ E-스포츠 중계]
시간: %s
사이온이 전력 질주를 시작합니다!
타겟: %s (거리 %.1fm)
현재 속도: %.1f m/s
배터리: %d%%

마치 경기를 중계하듯 열정적으로 설명해주세요!`,
				time.Now().Format("15:04:05"),
				targetName,
				dist,
				agvStatus.Speed,
				agvStatus.Battery)
		}

	case "kill":
		if agvStatus != nil {
			userPrompt = fmt.Sprintf(`[킬 확정! 🎉]
시간: %s
사이온이 적을 격파했습니다!
위치: (%.1f, %.1f)
남은 배터리: %d%%

격렬한 한국 e스포츠 해설 톤으로 축하 인사!`,
				time.Now().Format("15:04:05"),
				agvStatus.Position.X,
				agvStatus.Position.Y,
				agvStatus.Battery)
		}

	case "low_battery":
		if agvStatus != nil {
			userPrompt = fmt.Sprintf(`[위험! 배터리 부족 ⚠️]
시간: %s
사이온의 배터리가 %d%%로 매우 위험한 상황입니다!
위치: (%.1f, %.1f)

긴장감 있게 위험한 상황을 표현해주세요!`,
				time.Now().Format("15:04:05"),
				agvStatus.Battery,
				agvStatus.Position.X,
				agvStatus.Position.Y)
		}

	case "multiple_enemies":
		if agvStatus != nil && len(agvStatus.DetectedEnemies) > 0 {
			enemyCount := len(agvStatus.DetectedEnemies)
			userPrompt = fmt.Sprintf(`[다중 전투! 전장 상황 🔥]
시간: %s
사이온이 %d마리의 적에게 포위됐습니다!
현재 위치: (%.1f, %.1f)
배터리: %d%%

혼전 상황을 실시간 중계하듯 표현해주세요!`,
				time.Now().Format("15:04:05"),
				enemyCount,
				agvStatus.Position.X,
				agvStatus.Position.Y,
				agvStatus.Battery)
		}

	default:
		userPrompt = fmt.Sprintf("[이벤트: %s] 현재 상황을 e스포츠 해설처럼 열정적으로 설명해주세요.", eventType)
	}

	if userPrompt == "" {
		userPrompt = fmt.Sprintf("[이벤트: %s] 현재 상황을 설명해주세요.", eventType)
	}

	return s.callOllama(systemPrompt, userPrompt)
}

// analyzeTacticalSituation - 현재 전략적 상황 분석
func (s *LLMService) analyzeTacticalSituation(status *models.AGVStatus, battery int, enemyCount int) string {
	if enemyCount == 0 {
		return "안전한 상황입니다. 공격적의 플레이가 가능합니다!"
	}

	if battery < 30 {
		if enemyCount >= 2 {
			return "매우 위험한 상황입니다! 배터리 부족 + 다중 전투. 철수를 검토하세요."
		}
		return "배터리가 부족합니다. 신중하게 행동하세요."
	}

	if enemyCount >= 3 {
		return fmt.Sprintf("전략이 5:3으로 열위입니다! %d마리의 적에게 포위됐습니다. 빠른 처리 또는 철수 필요.",
			enemyCount)
	}

	if enemyCount >= 2 {
		if battery >= 70 {
			return fmt.Sprintf("2:2 상황입니다. 배터리 충분. 공격적의 플레이 가능! %d마리 격파 목표.",
				enemyCount)
		}
		return fmt.Sprintf("2:2 상황. 배터리 %d%%. 신중한 접근 필요.",
			battery)
	}

	// enemyCount == 1
	if battery >= 60 {
		return "5:1 상황입니다. 압도적 우위! 단일 적을 빠르게 제거하세요."
	}
	return "1:1 상황. 상황을 신중하게 판단하세요."
}

// evaluateTargetPriority - 타겟의 우선순위 평가
func (s *LLMService) evaluateTargetPriority(status *models.AGVStatus) string {
	if status.TargetEnemy == nil {
		return "없음"
	}

	targetHP := status.TargetEnemy.HP
	dist := calculateDistance(status.Position, status.TargetEnemy.Position)

	// 거리와 체력을 고려한 우선순위 판단
	if targetHP <= 30 && dist <= 5 {
		return "최상 (낮은 체력 + 근거리)"
	}
	if targetHP <= 20 {
		return "높음 (매우 낮은 체력)"
	}
	if dist <= 3 {
		return "높음 (매우 근거리)"
	}
	if targetHP >= 80 {
		return "낮음 (높은 체력)"
	}
	return "중간"
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