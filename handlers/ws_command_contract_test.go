package handlers

import (
	"encoding/json"
	"sion-backend/contract"
	"sion-backend/models"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

// 받아들인 명령에는 응답이 없으므로, 뒤이어 보낸 probe의 오류가 첫 오류로 오는지로 확인한다.
const probeError = `지원하지 않는 메시지 타입: "probe"`

var commandFixtureFiles = []string{"emergency_stop", "mode_change", "command"}

type acceptedCommand struct {
	Name    string          `json:"name"`
	Message json.RawMessage `json:"message"`
	Then    struct {
		Mode    *string `json:"mode"`
		State   *string `json:"state"`
		Battery *int    `json:"battery"`
	} `json:"then"`
}

type rejectedCommand struct {
	Name    string          `json:"name"`
	Message json.RawMessage `json:"message"`
	Raw     string          `json:"raw"`
	Error   string          `json:"error"`
}

func (r rejectedCommand) wire() string {
	if r.Message != nil {
		return string(r.Message)
	}
	return r.Raw
}

func dialReady(t *testing.T) (*wsTestServer, *websocket.Conn) {
	t.Helper()
	srv := newWSTestServer(t)
	web := srv.dial(t, "/websocket/web")
	readUntilType(t, web, models.MessageTypeMapUpdate, time.Second)
	return srv, web
}

func sendRaw(t *testing.T, c *websocket.Conn, raw string) {
	t.Helper()
	if err := c.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func firstErrorMessage(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	_, raw := readRawUntilType(t, c, models.MessageTypeError, time.Second)
	contract.Validate(t, "ErrorMessage", raw)
	var msg struct {
		Data models.ErrorData `json:"data"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	return msg.Data.Message
}

func expectRejected(t *testing.T, r rejectedCommand) {
	t.Helper()
	if r.Message != nil {
		if err := contract.Check(t, "ClientCommand", r.Message); err == nil {
			t.Fatalf("rejected fixture %s must violate the ClientCommand schema", r.Message)
		}
	}
	_, web := dialReady(t)
	sendRaw(t, web, r.wire())
	if got := firstErrorMessage(t, web); got != r.Error {
		t.Fatalf("error message = %q, want %q", got, r.Error)
	}
}

func TestWSCommandContract_Fixtures(t *testing.T) {
	for _, file := range commandFixtureFiles {
		var fixtures struct {
			Accepted []acceptedCommand `json:"accepted"`
			Rejected []rejectedCommand `json:"rejected"`
		}
		contract.LoadJSON(t, "fixtures/client/"+file+".json", &fixtures)
		if len(fixtures.Accepted) == 0 {
			t.Fatalf("%s.json has no accepted example", file)
		}

		for _, a := range fixtures.Accepted {
			t.Run(file+"/accepted/"+a.Name, func(t *testing.T) {
				contract.Validate(t, "ClientCommand", a.Message)
				srv, web := dialReady(t)
				sendRaw(t, web, string(a.Message))
				sendRaw(t, web, `{"type":"probe"}`)
				if got := firstErrorMessage(t, web); got != probeError {
					t.Fatalf("command should be accepted silently, got error %q", got)
				}
				status, _ := srv.sim.Snapshot()
				if a.Then.Mode != nil && status.Mode != *a.Then.Mode ||
					a.Then.State != nil && status.State != *a.Then.State ||
					a.Then.Battery != nil && status.Battery != *a.Then.Battery {
					t.Fatalf("simulator status mode=%q state=%q battery=%d does not match %+v", status.Mode, status.State, status.Battery, a.Then)
				}
			})
		}
		for _, r := range fixtures.Rejected {
			t.Run(file+"/rejected/"+r.Name, func(t *testing.T) { expectRejected(t, r) })
		}
	}
}

func TestWSCommandContract_InvalidMessages(t *testing.T) {
	var fixtures []rejectedCommand
	contract.LoadJSON(t, "fixtures/client/invalid.json", &fixtures)
	for _, r := range fixtures {
		t.Run(r.Name, func(t *testing.T) { expectRejected(t, r) })
	}
}

func TestWSCommandContract_ResetAfterEmergency(t *testing.T) {
	srv, web := dialReady(t)
	sendRaw(t, web, `{"type":"emergency_stop"}`)
	sendRaw(t, web, `{"type":"command","data":{"path":[{"x":3,"y":3}]}}`)
	if got := firstErrorMessage(t, web); got != "정지 상태입니다. 초기화 후 다시 시도하세요" {
		t.Fatalf("path during emergency: error = %q", got)
	}
	sendRaw(t, web, `{"type":"command","data":{"action":"reset"}}`)
	sendRaw(t, web, `{"type":"probe"}`)
	if got := firstErrorMessage(t, web); got != probeError {
		t.Fatalf("reset should be accepted silently, got error %q", got)
	}
	status, _ := srv.sim.Snapshot()
	if status.State == models.StateEmergency || status.Mode != models.ModeAuto || status.Battery != 100 {
		t.Fatalf("reset did not restore initial state: mode=%q state=%q battery=%d", status.Mode, status.State, status.Battery)
	}
}
