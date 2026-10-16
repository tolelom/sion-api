package handlers

import (
	"sion-backend/models"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

func TestWS_WelcomeSendsSystemInfoAndMap(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	info := readUntilType(t, web, models.MessageTypeSystemInfo, time.Second)
	if info.Data.(map[string]any)["agv_connected"] != true {
		t.Fatalf("the simulated AGV should be reported as connected: %v", info.Data)
	}
	m := readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	if obs, _ := m.Data.(map[string]any)["obstacles"].([]any); len(obs) == 0 {
		t.Fatalf("map update without obstacles: %v", m.Data)
	}
}

func TestWS_EmergencyStopReachesSimulator(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	sendJSON(t, web, map[string]any{"type": "emergency_stop", "data": map[string]any{"reason": "test"}})
	waitFor(t, time.Second, func() bool {
		status, _, _, _ := srv.sim.Snapshot()
		return status.State == models.StateEmergency
	}, "emergency stop did not reach the simulator")
}

func TestWS_RejectedCommandReturnsError(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	sendJSON(t, web, map[string]any{"type": "chat", "data": map[string]any{"message": "hi"}})
	got := readUntilType(t, web, models.MessageTypeError, time.Second)
	if got.Data.(map[string]any)["message"] == nil {
		t.Fatalf("error without message: %v", got.Data)
	}
}

func TestWS_InvalidJSONReturnsError(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	if err := web.WriteMessage(websocket.TextMessage, []byte("not-json")); err != nil {
		t.Fatal(err)
	}
	readUntilType(t, web, models.MessageTypeError, time.Second)
}

func TestWS_DisconnectUnregistersClient(t *testing.T) {
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	waitFor(t, time.Second, func() bool { return srv.cm.Count() == 1 }, "client not registered")
	web.Close()
	waitFor(t, time.Second, func() bool { return srv.cm.Count() == 0 }, "client not unregistered after close")
}
