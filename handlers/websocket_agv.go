package handlers

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/websocket/v2"
)

func NewAGVHandler(cm *services.ClientManager, broker *services.Broker) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		cm.Register(c, services.AGVClient)
		// defer는 LIFO: ① stopKeepalive로 ping 고루틴 종료
		// → ② SetAGVConnected(false)로 web에 disconnect 브로드캐스트
		// → ③ Unregister(c)로 자기 자신을 풀에서 제거. 자신의 conn에 마지막 알림이 가지 않게 한다.
		defer cm.Unregister(c)
		defer broker.SetAGVConnected(false)
		stopKeepalive := installKeepalive(cm, c, "AGV")
		defer stopKeepalive()

		broker.SetAGVConnected(true)

		for {
			_, p, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[WARN] AGV 비정상 종료: %v", err)
				} else {
					log.Println("[INFO] AGV 연결 종료")
				}
				break
			}

			var msg models.WebSocketMessage
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Printf("[WARN] AGV 메시지 파싱 오류: %v", err)
				continue
			}

			if msg.Timestamp == 0 {
				msg.Timestamp = time.Now().UnixMilli()
				updated, err := json.Marshal(msg)
				if err != nil {
					log.Printf("[WARN] AGV 메시지 timestamp 보정 marshal 실패: %v", err)
					continue
				}
				p = updated
			}

			go services.LogAGVEvent(msg, "sion-001", "agv")
			log.Printf("[INFO] AGV 메시지: %s", msg.Type)
			broker.OnAGVMessage(msg, p)
		}
	}
}
