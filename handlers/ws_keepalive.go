package handlers

import (
	"log"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

// WebSocket keepalive 파라미터.
// pongWait: 마지막 pong 이후 read를 기다리는 최대 시간. 초과 시 ReadMessage가 timeout으로 깨어나 핸들러를 종료시킨다.
// pingPeriod: ping 송신 주기. pongWait보다 작아야 클라이언트가 pong을 보낼 여유가 생긴다 (관례상 9/10).
// writeWait: 컨트롤 프레임 한 번의 송신 deadline.
//
// var로 둔 이유: 테스트에서 짧은 시간으로 교체해 keepalive 타임아웃 시나리오를 검증하기 위함.
// 호출자(installKeepalive)는 호출 시점에 값을 캡처하므로 핸들러 시작 전에만 교체하면 된다.
var (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
)

// installKeepalive는 conn에 read deadline·pong 핸들러를 설치하고
// 별도 goroutine에서 주기적으로 ping을 보내는 keepalive 루틴을 시작한다.
// 호출자는 read 루프 종료 직전에 반환된 stop 함수를 호출해 ping goroutine을 정리해야 한다.
func installKeepalive(cm *services.ClientManager, c *websocket.Conn, label string) (stop func()) {
	if err := c.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("[WARN] %s SetReadDeadline 실패: %v", label, err)
	}
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(pongWait))
	})

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := cm.WriteControl(c, websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
					log.Printf("[INFO] %s ping 실패, keepalive 종료: %v", label, err)
					return
				}
			case <-stopCh:
				return
			}
		}
	}()

	return func() {
		close(stopCh)
		<-doneCh
	}
}
