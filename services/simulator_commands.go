package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sion-backend/models"
)

func (sim *AGVSimulator) HandleCommand(msg models.WebSocketMessage) error {
	switch msg.Type {
	case models.MessageTypeEmergencyStop:
		sim.mu.Lock()
		sim.status.State = models.StateEmergency
		sim.status.Speed = 0
		sim.path = nil
		sim.mu.Unlock()
		return nil
	case models.MessageTypeModeChange:
		var cmd struct {
			Mode string `json:"mode"`
		}
		if err := decodeData(msg.Data, &cmd); err != nil {
			return err
		}
		if cmd.Mode != models.ModeAuto && cmd.Mode != models.ModeManual {
			return fmt.Errorf("알 수 없는 모드: %q", cmd.Mode)
		}
		sim.mu.Lock()
		sim.status.Mode = cmd.Mode
		sim.path = nil
		sim.mu.Unlock()
		return nil
	case models.MessageTypeCommand:
		return sim.handleMoveCommand(msg.Data)
	default:
		return fmt.Errorf("지원하지 않는 메시지 타입: %q", msg.Type)
	}
}

func (sim *AGVSimulator) handleMoveCommand(data any) error {
	var cmd struct {
		Action string            `json:"action"`
		Path   []models.Waypoint `json:"path"`
	}
	if err := decodeData(data, &cmd); err != nil {
		return err
	}
	if cmd.Action == "reset" {
		sim.mu.Lock()
		sim.resetLocked()
		sim.mu.Unlock()
		return nil
	}
	if len(cmd.Path) == 0 {
		return errors.New("경로가 비어 있습니다")
	}
	if len(cmd.Path) > maxPathPoints {
		return fmt.Errorf("경로는 최대 %d개 지점까지입니다", maxPathPoints)
	}
	for _, p := range cmd.Path {
		if p.X < 0 || p.Y < 0 || p.X > MapSize || p.Y > MapSize {
			return errors.New("경로가 맵을 벗어납니다")
		}
	}
	sim.mu.Lock()
	defer sim.mu.Unlock()
	if sim.status.State == models.StateEmergency || sim.status.Battery == 0 {
		return errors.New("정지 상태입니다. 초기화 후 다시 시도하세요")
	}
	sim.status.Mode = models.ModeManual
	sim.path = cmd.Path
	return nil
}

func decodeData(data any, dst any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return errors.New("잘못된 명령 형식")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return errors.New("잘못된 명령 형식")
	}
	return nil
}
