package handlers

import (
	"log"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

// keepalive는 WebSocket keepalive 파라미터다.
// pongWait: 마지막 pong 이후 read를 기다리는 최대 시간. 초과 시 ReadMessage가 timeout으로 깨어나 핸들러를 종료시킨다.
// pingPeriod: ping 송신 주기. pongWait보다 작아야 클라이언트가 pong을 보낼 여유가 생긴다 (관례상 9/10).
// writeWait: 컨트롤 프레임 한 번의 송신 deadline.
//
// 핸들러를 만들 때 값으로 넘겨, 테스트가 짧은 값을 써도 다른 테스트의 핸들러와 전역을 공유하지 않는다.
type keepalive struct {
	pongWait   time.Duration
	pingPeriod time.Duration
	writeWait  time.Duration
}

var defaultKeepalive = keepalive{
	pongWait:   60 * time.Second,
	pingPeriod: 54 * time.Second,
	writeWait:  10 * time.Second,
}

// install은 conn에 read deadline·pong 핸들러를 설치하고
// 별도 goroutine에서 주기적으로 ping을 보내는 keepalive 루틴을 시작한다.
// 호출자는 read 루프 종료 직전에 반환된 stop 함수를 호출해 ping goroutine을 정리해야 한다.
func (k keepalive) install(cm *services.ClientManager, c *websocket.Conn, label string) (stop func()) {
	if err := c.SetReadDeadline(time.Now().Add(k.pongWait)); err != nil {
		log.Printf("[WARN] %s SetReadDeadline 실패: %v", label, err)
	}
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(k.pongWait))
	})

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(k.pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := cm.WriteControl(c, websocket.PingMessage, nil, time.Now().Add(k.writeWait)); err != nil {
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
