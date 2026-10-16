package handlers

import (
	"bytes"
	"sion-backend/services"
	"testing"
	"time"
)

func TestBroadcast_StuckClientDoesNotStallOthers(t *testing.T) {
	cm := services.NewClientManager()
	cm.SetWriteTimeout(200 * time.Millisecond)
	srv := newWSTestServerWith(t, cm)

	srv.dial(t, "/websocket/web")
	healthy := srv.dial(t, "/websocket/web")
	waitWebClients(t, cm, 2)

	payload := bytes.Repeat([]byte("x"), 1<<20)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 30; i++ {
			cm.BroadcastToWeb(payload)
		}
	}()
	go func() {
		for {
			healthy.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, _, err := healthy.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("broadcasts stalled behind a client that never reads")
	}
	if n := cm.Count(); n != 1 {
		t.Fatalf("the stuck client should be dropped, %d web clients remain", n)
	}
}

func waitWebClients(t *testing.T, cm *services.ClientManager, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for cm.Count() < n {
		if time.Now().After(deadline) {
			t.Fatalf("only %d web clients registered", cm.Count())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
