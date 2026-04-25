package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// callOllama는 system+user 프롬프트를 합쳐 Ollama /api/generate에 POST하고 응답 텍스트를 반환한다.
// HTTP 상태/디코딩/빈 응답을 모두 에러로 분리해 호출자가 원인을 식별할 수 있게 한다.
func (s *LLMService) callOllama(systemPrompt, userPrompt string) (string, error) {
	start := time.Now()
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

	client := &http.Client{Timeout: time.Duration(s.TimeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama 호출 실패: %v", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama 응답 읽기 실패: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(b))
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

	log.Printf("[INFO] Ollama 응답 시간: %.2fs (model=%s)", time.Since(start).Seconds(), s.Model)
	return result.Response, nil
}
