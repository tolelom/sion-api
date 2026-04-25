package services

import (
	"sion-backend/models"
	"strings"
	"testing"
)

func TestAnalyzeTacticalSituation(t *testing.T) {
	cases := []struct {
		name       string
		battery    int
		enemyCount int
		want       string
	}{
		{"적 없음 → 안전", 100, 0, "안전"},
		{"낮은 배터리 + 다수 적 → 매우 위험", 20, 2, "매우 위험"},
		{"낮은 배터리 + 1명 → 위험", 20, 1, "위험"},
		{"배터리 충분 + 다수 적 → 열세", 80, 3, "열세"},
		{"고배터리 + 1명 → 유리", 80, 1, "유리"},
		{"중간 케이스 → 경계", 50, 2, "경계"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := analyzeTacticalSituation(tc.battery, tc.enemyCount)
			if got != tc.want {
				t.Fatalf("battery=%d enemies=%d → %q 기대, got %q",
					tc.battery, tc.enemyCount, tc.want, got)
			}
		})
	}
}

func TestBuildAnswerQuestionPrompt_NilStatus(t *testing.T) {
	prompt := buildAnswerQuestionPrompt("배터리 어때?", nil, "")
	if !strings.Contains(prompt, "배터리 어때?") {
		t.Fatalf("질문 포함 기대, got %q", prompt)
	}
	if !strings.Contains(prompt, "상태 정보는 없지만") {
		t.Fatalf("nil 상태 폴백 멘트 기대, got %q", prompt)
	}
}

func TestBuildAnswerQuestionPrompt_WithTarget(t *testing.T) {
	status := &models.AGVStatus{
		Position: models.PositionData{X: 1.5, Y: 2.5},
		Battery:  77,
		DetectedEnemies: []models.Enemy{
			{ID: "e1"},
			{ID: "e2"},
		},
		TargetEnemy: &models.Enemy{Name: "아리", HP: 42},
	}
	prompt := buildAnswerQuestionPrompt("뭐해?", status, "경계")
	for _, want := range []string{"(1.5, 2.5)", "77%", "2명", "경계", "아리", "42%"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("프롬프트에 %q 포함 기대, got %q", want, prompt)
		}
	}
}

func TestBuildExplainEventPrompt_KnownEvents(t *testing.T) {
	statusWithTarget := &models.AGVStatus{
		Position: models.PositionData{X: 0, Y: 0},
		Battery:  15,
		DetectedEnemies: []models.Enemy{
			{ID: "e1"},
			{ID: "e2"},
		},
		TargetEnemy: &models.Enemy{
			Name:     "아리",
			Position: models.PositionData{X: 3, Y: 4},
		},
	}

	t.Run("target_change", func(t *testing.T) {
		got := buildExplainEventPrompt("target_change", statusWithTarget)
		if !strings.Contains(got, "아리") || !strings.Contains(got, "5.0m") {
			t.Fatalf("타겟명/거리 포함 기대, got %q", got)
		}
	})

	t.Run("low_battery", func(t *testing.T) {
		got := buildExplainEventPrompt("low_battery", statusWithTarget)
		if !strings.Contains(got, "15%") {
			t.Fatalf("배터리 수치 포함 기대, got %q", got)
		}
	})

	t.Run("multiple_enemies", func(t *testing.T) {
		got := buildExplainEventPrompt("multiple_enemies", statusWithTarget)
		if !strings.Contains(got, "2명") {
			t.Fatalf("적 수 포함 기대, got %q", got)
		}
	})

	t.Run("kill — 상태 무관", func(t *testing.T) {
		got := buildExplainEventPrompt("kill", nil)
		if !strings.Contains(got, "잡았어요") {
			t.Fatalf("kill 멘트 기대, got %q", got)
		}
	})
}

func TestBuildExplainEventPrompt_UnknownEventFallback(t *testing.T) {
	got := buildExplainEventPrompt("zzz_unknown", nil)
	if !strings.Contains(got, "zzz_unknown") {
		t.Fatalf("unknown 이벤트 타입 포함 기대, got %q", got)
	}
}

func TestBuildExplainEventPrompt_TargetChangeWithoutTarget(t *testing.T) {
	// TargetEnemy 없는 상태에서 target_change → fallback 멘트
	got := buildExplainEventPrompt("target_change", &models.AGVStatus{})
	if !strings.Contains(got, "target_change") {
		t.Fatalf("타겟 없을 때 unknown 폴백 기대, got %q", got)
	}
}
