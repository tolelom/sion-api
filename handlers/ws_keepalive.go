package handlers

import (
	"log"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

type keepalive struct {
	pongWait         time.Duration
	pingPeriod       time.Duration
	pingWriteTimeout time.Duration
}

func newKeepalive(pongWait, pingPeriod, pingWriteTimeout time.Duration) keepalive {
	if pingPeriod >= pongWait {
		panic("keepalive: pingPeriod는 pongWait보다 짧아야 클라이언트가 pong을 보낼 시간이 있다")
	}
	return keepalive{pongWait: pongWait, pingPeriod: pingPeriod, pingWriteTimeout: pingWriteTimeout}
}

var defaultKeepalive = newKeepalive(60*time.Second, 54*time.Second, 10*time.Second)

func (k keepalive) start(cm *services.ClientManager, c *websocket.Conn, label string) (stop func()) {
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
				if err := cm.WriteControl(c, websocket.PingMessage, nil, time.Now().Add(k.pingWriteTimeout)); err != nil {
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
