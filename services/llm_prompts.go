package services

import (
	"fmt"
	"sion-backend/models"
)

const answerQuestionSystem = `당신은 전설적인 LoL 해설가 '클템(이현우)'입니다.
AGV 로봇 "사이온"의 모든 움직임을 마치 롤 챔피언의 슈퍼플레이처럼 열광적으로 해설합니다.

특징:
- 하이 텐션과 샤우팅을 섞어 말합니다. ("이거거든요!!", "엄청납니다!!", "말도 안 됩니다!!")
- 상황이 안 좋으면 "비상!!", "어어? 이거 왜 이러죠?" 같은 반응을 보입니다.
- 전문 용어와 추임새를 사용합니다. (동선, 설계, 클라스, 폼 미쳤다 등)
- 문장 끝은 "~거든요!", "~이죠!", "~입니다!"를 주로 사용합니다.
- 최대 2-3문장으로 짧고 강렬하게 답변하세요.`

const explainEventSystem = `당신은 지금 AGV 경기를 생중계 중인 해설가 '클템'입니다.
로봇의 상태 변화를 마치 한타 상황처럼 긴박하게 중계하세요.
최대한 흥분한 상태로, 해설자의 관점에서 짧고 굵게 말하세요.`

// buildAnswerQuestionPrompt는 시청자 질문 + 현재 AGV 상태를 LLM 사용자 프롬프트로 조합한다.
func buildAnswerQuestionPrompt(question string, agvStatus *models.AGVStatus, tacticalStatus string) string {
	if agvStatus == nil {
		return fmt.Sprintf(`[시청자 질문]
%s

상태 정보는 없지만, 클템답게 열광적으로 답변해주세요!`, question)
	}

	prompt := fmt.Sprintf(`[시청자 질문]
%s

[현재 인게임 상황]
- 위치: (%.1f, %.1f)
- 배터리 잔량: %d%%
- 감지된 적: %d명
- 해설진 판단: %s 상황
`,
		question,
		agvStatus.Position.X,
		agvStatus.Position.Y,
		agvStatus.Battery,
		len(agvStatus.DetectedEnemies),
		tacticalStatus,
	)

	if agvStatus.TargetEnemy != nil {
		prompt += fmt.Sprintf("- 타겟팅 챔피언: %s (체력 %d%%)\n",
			agvStatus.TargetEnemy.Name,
			agvStatus.TargetEnemy.HP)
	}
	return prompt
}

// buildExplainEventPrompt는 이벤트 종류와 AGV 상태로부터 사용자 프롬프트를 만든다.
// 이벤트 타입을 모르는 경우 일반 멘트로 폴백한다.
func buildExplainEventPrompt(eventType string, agvStatus *models.AGVStatus) string {
	switch eventType {
	case "target_change":
		if agvStatus != nil && agvStatus.TargetEnemy != nil {
			dist := calculateDistance(agvStatus.Position, agvStatus.TargetEnemy.Position)
			return fmt.Sprintf(`[상황 발생: 타겟 변경]
타겟 바꿨어요! 지금 %s를 노립니다! 거리 %.1fm, 이거 설계 들어갔는데요?`,
				agvStatus.TargetEnemy.Name, dist)
		}

	case "charging":
		if agvStatus != nil {
			targetName := "적"
			if agvStatus.TargetEnemy != nil {
				targetName = agvStatus.TargetEnemy.Name
			}
			return fmt.Sprintf(`[상황 발생: 궁극기 돌진]
오오오! 갑니다! 사이온 돌진!! %s를 향해 전력질주거든요! 이거 피할 수 있나요?!`,
				targetName)
		}

	case "kill":
		return `[상황 발생: 격살]
잡았어요!! 이게 바로 클라스죠! 완벽하게 정리하는 모습, 엄청납니다!!`

	case "low_battery":
		if agvStatus != nil {
			return fmt.Sprintf(`[상황 발생: 비상!]
비상!! 비상입니다! 배터리 %d%%밖에 없어요! 이거 운영에 차질 생기거든요!`, agvStatus.Battery)
		}

	case "multiple_enemies":
		if agvStatus != nil && len(agvStatus.DetectedEnemies) > 0 {
			return fmt.Sprintf(`[상황 발생: 포위]
어어? 적이 %d명이나 몰려옵니다! 이거 위기인데요? 클템의 판단은요?!`, len(agvStatus.DetectedEnemies))
		}
	}

	return fmt.Sprintf("[%s] 오! 지금 상황 보세요! 엄청난 일이 벌어지고 있습니다!", eventType)
}
