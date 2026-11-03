package handlers

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
)

// 풀로 돌아간 *Conn에 브로드캐스트가 쓰면 nil 역참조로 서버 전체가 죽는다.
func TestBroadcastDuringDisconnectIsSafe(t *testing.T) {
	srv := newWSTestServer(t)

	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			srv.cm.BroadcastJSON(map[string]string{"type": "probe"})
		}
	}()

	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	var clients sync.WaitGroup
	for i := 0; i < 4; i++ {
		clients.Add(1)
		go func() {
			defer clients.Done()
			for j := 0; j < 25; j++ {
				c, _, err := dialer.Dial("ws://"+srv.addr+"/websocket/web", nil)
				if err != nil {
					t.Errorf("dial: %v", err)
					return
				}
				c.SetReadDeadline(time.Now().Add(time.Second))
				_, _, _ = c.ReadMessage()
				_ = c.Close()
			}
		}()
	}
	clients.Wait()
	stop.Store(true)
	wg.Wait()
	waitFor(t, 2*time.Second, func() bool { return srv.cm.Count() == 0 }, "all clients unregistered")
}
