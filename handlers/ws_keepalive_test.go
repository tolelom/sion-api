package handlers

import (
	"sion-backend/services"
	"testing"
	"time"
)

var shortKeepalive = newKeepalive(200*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)

func TestWS_KeepaliveDropsSilentClient(t *testing.T) {
	srv := newWSTestServerWith(t, services.NewClientManager(services.DefaultWriteTimeout), shortKeepalive)
	web := srv.dial(t, "/websocket/web")
	web.SetPingHandler(func(string) error { return nil })
	waitFor(t, time.Second, func() bool { return srv.cm.Count() == 1 }, "client not registered")
	go func() {
		for {
			if _, _, err := web.ReadMessage(); err != nil {
				return
			}
		}
	}()
	waitFor(t, 2*time.Second, func() bool { return srv.cm.Count() == 0 }, "silent client was not dropped after pongWait")
}

func TestWS_KeepaliveKeepsRespondingClient(t *testing.T) {
	srv := newWSTestServerWith(t, services.NewClientManager(services.DefaultWriteTimeout), shortKeepalive)
	web := srv.dial(t, "/websocket/web")
	go func() {
		for {
			if _, _, err := web.ReadMessage(); err != nil {
				return
			}
		}
	}()
	waitFor(t, time.Second, func() bool { return srv.cm.Count() == 1 }, "client not registered")
	end := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(end) {
		if srv.cm.Count() != 1 {
			t.Fatal("client answering pings was dropped")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
