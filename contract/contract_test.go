package contract

import "testing"

func TestServerMessagesRejectUndeclaredFields(t *testing.T) {
	declared := `{"type":"error","data":{"message":"x"},"timestamp":1}`
	Validate(t, "ServerMessage", []byte(declared))
	for _, raw := range []string{
		`{"type":"error","data":{"message":"x","code":1},"timestamp":1}`,
		`{"type":"error","data":{"message":"x"},"timestamp":1,"id":"a"}`,
		`{"type":"path_update","data":{},"timestamp":1}`,
	} {
		if Check(t, "ServerMessage", []byte(raw)) == nil {
			t.Errorf("schema should reject %s", raw)
		}
	}
}
