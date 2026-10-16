package handlers

import (
	"testing"
	"time"
)

func shortenKeepalive(t *testing.T, pong, ping, write time.Duration) {
	t.Helper()
	oldPong, oldPing, oldWrite := pongWait, pingPeriod, writeWait
	pongWait, pingPeriod, writeWait = pong, ping, write
	t.Cleanup(func() { pongWait, pingPeriod, writeWait = oldPong, oldPing, oldWrite })
}

func TestWS_KeepaliveDropsSilentClient(t *testing.T) {
	shortenKeepalive(t, 200*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)
	srv := newWSTestServer(t)
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
	shortenKeepalive(t, 200*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)
	srv := newWSTestServer(t)
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
