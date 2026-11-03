package services

import (
	"encoding/json"
	"reflect"
	"sion-backend/contract"
	"sion-backend/models"
	"testing"
)

var messageDefs = map[string]string{
	models.MessageTypeStatus:     "StatusMessage",
	models.MessageTypePosition:   "PositionMessage",
	models.MessageTypeMapUpdate:  "MapUpdateMessage",
	models.MessageTypeSystemInfo: "SystemInfoMessage",
	models.MessageTypeError:      "ErrorMessage",
}

func validateBroadcast(t *testing.T, msg models.WebSocketMessage) {
	t.Helper()
	def, ok := messageDefs[msg.Type]
	if !ok {
		t.Fatalf("message type %q is not in the contract", msg.Type)
	}
	contract.ValidateValue(t, def, msg)
	contract.ValidateValue(t, "ServerMessage", msg)
}

func TestContract_SimulatorBroadcastsMatchSchema(t *testing.T) {
	sim, _ := newTestSim(true)
	validateBroadcast(t, sim.MapUpdateMessage())

	stepAndValidate := func(n int) {
		for i := 0; i < n; i++ {
			status, position := sim.step()
			validateBroadcast(t, status)
			validateBroadcast(t, position)
		}
	}
	stepAndValidate(5)

	sim.mu.Lock()
	sim.enemies[0].Position = models.PositionData{X: startX + 1, Y: startY + 1}
	sim.mu.Unlock()
	status, position := sim.step()
	validateBroadcast(t, status)
	validateBroadcast(t, position)
	if status.Data.(models.StatusData).TargetEnemy == nil {
		t.Fatal("an enemy next to the AGV should become the target")
	}
	stepAndValidate(5)

	sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"path": []map[string]float64{{"x": 3, "y": 3}}}))
	stepAndValidate(10)
	sim.HandleCommand(command(t, models.MessageTypeEmergencyStop, nil))
	stepAndValidate(2)

	sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"action": "reset"}))
	sim.mu.Lock()
	sim.status.Battery = 0
	sim.mu.Unlock()
	stepAndValidate(2)
}

type fixture struct {
	Name    string          `json:"name"`
	Message json.RawMessage `json:"message"`
}

var payloadTypes = map[string]func() any{
	models.MessageTypeStatus:     func() any { return &models.StatusData{} },
	models.MessageTypePosition:   func() any { return &models.PositionData{} },
	models.MessageTypeMapUpdate:  func() any { return &models.MapUpdateData{} },
	models.MessageTypeSystemInfo: func() any { return &models.SystemInfoData{} },
	models.MessageTypeError:      func() any { return &models.ErrorData{} },
}

func TestContract_ServerFixturesRoundTripThroughGoTypes(t *testing.T) {
	for msgType, def := range messageDefs {
		var fixtures []fixture
		contract.LoadJSON(t, "fixtures/server/"+msgType+".json", &fixtures)
		if len(fixtures) == 0 {
			t.Fatalf("no fixtures for %s", msgType)
		}
		for _, f := range fixtures {
			t.Run(msgType+"/"+f.Name, func(t *testing.T) {
				contract.Validate(t, def, f.Message)

				var envelope struct {
					Type      string          `json:"type"`
					Data      json.RawMessage `json:"data"`
					Timestamp int64           `json:"timestamp"`
				}
				if err := json.Unmarshal(f.Message, &envelope); err != nil {
					t.Fatal(err)
				}
				if envelope.Type != msgType {
					t.Fatalf("fixture in %s.json has type %q", msgType, envelope.Type)
				}
				payload := payloadTypes[msgType]()
				if err := json.Unmarshal(envelope.Data, payload); err != nil {
					t.Fatalf("Go type rejects fixture data: %v", err)
				}
				out, err := json.Marshal(models.WebSocketMessage{Type: envelope.Type, Data: payload, Timestamp: envelope.Timestamp})
				if err != nil {
					t.Fatal(err)
				}
				assertSameJSON(t, out, f.Message)
			})
		}
	}
}

func assertSameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("Go types produce\n%s\nwant\n%s", got, want)
	}
}
