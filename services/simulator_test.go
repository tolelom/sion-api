package services

import (
	"encoding/json"
	"math"
	"sion-backend/models"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu   sync.Mutex
	msgs []models.WebSocketMessage
}

func (r *recorder) add(m models.WebSocketMessage) {
	r.mu.Lock()
	r.msgs = append(r.msgs, m)
	r.mu.Unlock()
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.msgs)
}

func newTestSim(listening bool) (*AGVSimulator, *recorder) {
	rec := &recorder{}
	sim := NewAGVSimulator(rec.add, func() bool { return listening })
	return sim, rec
}

func command(t *testing.T, msgType string, data any) models.WebSocketMessage {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	json.Unmarshal(raw, &decoded)
	return models.WebSocketMessage{Type: msgType, Data: decoded}
}

func ticks(sim *AGVSimulator, n int) {
	for i := 0; i < n; i++ {
		sim.update()
	}
}

func position(sim *AGVSimulator) models.PositionData {
	status, _, _, _ := sim.Snapshot()
	return status.Position
}

func TestSimulator_IdlesWithoutListeners(t *testing.T) {
	sim, rec := newTestSim(false)
	before := position(sim)
	ticks(sim, 20)
	if rec.count() != 0 || position(sim) != before {
		t.Fatal("simulator must not move or broadcast while nobody is watching")
	}
}

func TestSimulator_EmergencyStopHoldsUntilReset(t *testing.T) {
	sim, _ := newTestSim(true)
	ticks(sim, 3)
	if err := sim.HandleCommand(command(t, models.MessageTypeEmergencyStop, map[string]any{"reason": "test"})); err != nil {
		t.Fatal(err)
	}
	stopped := position(sim)
	ticks(sim, 20)
	status, _, _, _ := sim.Snapshot()
	if position(sim) != stopped || status.State != models.StateEmergency || status.Speed != 0 {
		t.Fatalf("emergency stop must hold: state=%s speed=%v moved=%v", status.State, status.Speed, position(sim) != stopped)
	}
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"action": "reset"})); err != nil {
		t.Fatal(err)
	}
	ticks(sim, 3)
	if status, _, _, _ := sim.Snapshot(); status.State == models.StateEmergency {
		t.Fatal("reset must clear the emergency stop")
	}
}

func TestSimulator_ModeChangeValidatesMode(t *testing.T) {
	sim, _ := newTestSim(true)
	if err := sim.HandleCommand(command(t, models.MessageTypeModeChange, map[string]any{"mode": "manual"})); err != nil {
		t.Fatal(err)
	}
	if status, _, _, _ := sim.Snapshot(); status.Mode != models.ModeManual {
		t.Fatalf("mode = %s", status.Mode)
	}
	if err := sim.HandleCommand(command(t, models.MessageTypeModeChange, map[string]any{"mode": "turbo"})); err == nil {
		t.Fatal("unknown mode must be rejected")
	}
}

func TestSimulator_FollowsCommandedPath(t *testing.T) {
	sim, _ := newTestSim(true)
	start := position(sim)
	path := []map[string]float64{
		{"x": math.Round(start.X), "y": math.Round(start.Y)},
		{"x": math.Round(start.X) + 3, "y": math.Round(start.Y)},
		{"x": math.Round(start.X) + 3, "y": math.Round(start.Y) + 2},
	}
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"path": path, "mode": "manual"})); err != nil {
		t.Fatal(err)
	}
	ticks(sim, 60)
	end := position(sim)
	last := path[len(path)-1]
	if math.Hypot(end.X-last["x"], end.Y-last["y"]) > 0.2 {
		t.Fatalf("AGV should reach the path end %v, got (%.2f, %.2f)", last, end.X, end.Y)
	}
	if status, _, _, _ := sim.Snapshot(); status.Mode != models.ModeManual {
		t.Fatal("following a commanded path must switch to manual mode")
	}
	ticks(sim, 20)
	if after := position(sim); after != end {
		t.Fatal("AGV must stay at the end of a finished path")
	}
}

func TestSimulator_RejectsBadPaths(t *testing.T) {
	sim, _ := newTestSim(true)
	outside := []map[string]float64{{"x": 1, "y": 1}, {"x": 500, "y": 1}}
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"path": outside})); err == nil {
		t.Fatal("a path leaving the map must be rejected")
	}
	long := make([]map[string]float64, 1000)
	for i := range long {
		long[i] = map[string]float64{"x": 1, "y": 1}
	}
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"path": long})); err == nil {
		t.Fatal("an oversized path must be rejected")
	}
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{})); err == nil {
		t.Fatal("a command with neither path nor action must be rejected")
	}
}

func TestSimulator_ResetRestoresInitialState(t *testing.T) {
	sim, _ := newTestSim(true)
	sim.HandleCommand(command(t, models.MessageTypeModeChange, map[string]any{"mode": "manual"}))
	ticks(sim, 30)
	if err := sim.HandleCommand(command(t, models.MessageTypeCommand, map[string]any{"action": "reset"})); err != nil {
		t.Fatal(err)
	}
	status, enemies, _, _ := sim.Snapshot()
	if status.Mode != models.ModeAuto || status.Battery != 100 || status.Position.X != startX || status.Position.Y != startY {
		t.Fatalf("reset did not restore the start: %+v", status)
	}
	for _, e := range enemies {
		if e.HP <= 0 {
			t.Fatal("reset must respawn enemies")
		}
	}
}

func TestSimulator_DeadBatteryStopsUntilReset(t *testing.T) {
	sim, _ := newTestSim(true)
	sim.mu.Lock()
	sim.status.Battery = 1
	sim.mu.Unlock()
	ticks(sim, 30)
	stopped := position(sim)
	ticks(sim, 30)
	status, _, _, _ := sim.Snapshot()
	if status.Battery != 0 || status.Speed != 0 || status.State != models.StateStopped || position(sim) != stopped {
		t.Fatalf("dead battery must stop the AGV: battery=%d speed=%v state=%s", status.Battery, status.Speed, status.State)
	}
}

func TestSimulator_StaysOnMap(t *testing.T) {
	sim, _ := newTestSim(true)
	ticks(sim, 2000)
	p := position(sim)
	if p.X < 0 || p.Y < 0 || p.X > MapSize || p.Y > MapSize {
		t.Fatalf("AGV left the map: (%.2f, %.2f)", p.X, p.Y)
	}
}

func TestSimulator_RejectsUnknownMessages(t *testing.T) {
	sim, _ := newTestSim(true)
	if err := sim.HandleCommand(models.WebSocketMessage{Type: "chat"}); err == nil {
		t.Fatal("unknown message types must be rejected")
	}
}

func TestSimulator_MapUpdateListsObstaclesInsideMap(t *testing.T) {
	sim, _ := newTestSim(true)
	msg := sim.MapUpdateMessage()
	raw, _ := json.Marshal(msg.Data)
	var data struct {
		Obstacles     []struct{ X, Y int }
		Width, Height int
	}
	json.Unmarshal(raw, &data)
	if msg.Type != models.MessageTypeMapUpdate || data.Width != MapSize || data.Height != MapSize || len(data.Obstacles) == 0 {
		t.Fatalf("bad map update: %s %+v", msg.Type, data)
	}
	for _, o := range data.Obstacles {
		if o.X < 0 || o.Y < 0 || o.X >= MapSize || o.Y >= MapSize {
			t.Fatalf("obstacle outside map: %+v", o)
		}
	}
}

func TestSimulator_ConcurrentStartStopIsSafe(t *testing.T) {
	sim, _ := newTestSim(true)
	sim.updateInterval = time.Millisecond
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sim.Start()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sim.Stop()
			}
		}()
	}
	wg.Wait()
	sim.Stop()
	if sim.IsRunning() {
		t.Fatal("simulator still running after final Stop")
	}
}

func TestSimulator_SlowBroadcastDoesNotBlockCommands(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	sim := NewAGVSimulator(func(models.WebSocketMessage) {
		once.Do(func() { close(entered) })
		<-release
	}, func() bool { return true })
	sim.updateInterval = time.Millisecond
	sim.Start()
	defer func() {
		close(release)
		sim.Stop()
	}()
	<-entered

	got := make(chan struct{})
	go func() {
		sim.HandleCommand(models.WebSocketMessage{Type: models.MessageTypeEmergencyStop})
		sim.Snapshot()
		close(got)
	}()
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("commands blocked while a broadcast was in progress")
	}
}
