package handlers

import (
	"testing"
	"time"
)

// shortenKeepalive는 keepalive 파라미터를 짧게 교체하고 t.Cleanup으로 복원한다.
// 테스트가 60s pongWait를 그대로 기다리지 않게 한다.
func shortenKeepalive(t *testing.T, pong, ping, write time.Duration) {
	t.Helper()
	oldPong, oldPing, oldWrite := pongWait, pingPeriod, writeWait
	pongWait = pong
	pingPeriod = ping
	writeWait = write
	t.Cleanup(func() {
		pongWait = oldPong
		pingPeriod = oldPing
		writeWait = oldWrite
	})
}

// TestWS_KeepaliveTimeout_DisconnectsSilentClient
// 서버는 pingPeriod마다 ping을 보낸다. 클라이언트가 SetPingHandler를 빈 함수로 두면 pong을 안 돌려준다.
// pongWait이 지나면 서버 ReadDeadline이 만료되어 ReadMessage가 timeout 깨어나고
// 핸들러가 종료되며 broker.IsAGVConnected가 false로 복귀한다.
func TestWS_KeepaliveTimeout_DisconnectsSilentClient(t *testing.T) {
	shortenKeepalive(t, 200*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)

	srv := newWSTestServer(t)

	agv := srv.dial(t, "/websocket/agv")
	// 클라이언트가 ping을 받아도 pong을 자동으로 돌려보내지 않게 한다.
	agv.SetPingHandler(func(string) error { return nil })

	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV connected wait")

	// pongWait(200ms)가 지나면 서버는 read timeout으로 핸들러를 종료한다.
	// 여유를 두고 1초 안에 disconnect 반영을 기대.
	waitFor(t, 1*time.Second, func() bool { return !srv.broker.IsAGVConnected() },
		"keepalive timeout 후에도 broker가 연결된 상태로 남음")

	// agv.ReadMessage가 close 프레임을 수신하거나 EOF를 받을 때까지 한 번 폴링.
	// (이 케이스의 본질은 broker 상태 — read 결과까지는 강제 검증 안 함.)
	_ = agv.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, _ = agv.ReadMessage()
}

// TestWS_KeepalivePongResponse_StaysConnected
// 클라이언트가 ping에 정상적으로 pong을 보내면 핸들러는 살아 있어야 한다.
// fasthttp/websocket 클라이언트의 default ping handler가 pong을 보내므로 별도 설정 없음.
func TestWS_KeepalivePongResponse_StaysConnected(t *testing.T) {
	shortenKeepalive(t, 200*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)

	srv := newWSTestServer(t)

	agv := srv.dial(t, "/websocket/agv")
	waitFor(t, 1*time.Second, srv.broker.IsAGVConnected, "AGV connected wait")

	// pongWait(200ms)의 약 3배가 지나도 살아 있어야 한다.
	// pong이 수신되며 SetReadDeadline이 갱신돼 timeout이 발생하지 않는 흐름을 확인.
	// 클라이언트가 read 루프를 돌려야 ping 프레임이 처리되어 pong이 자동 송신된다.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = agv.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
		// ReadMessage가 control frame을 처리하면서 default ping handler가 pong을 보낸다.
		// 데이터 메시지가 없으면 deadline에서 timeout으로 빠져나온다.
		_, _, _ = agv.ReadMessage()
	}()

	// 핸들러가 살아 있는 동안 broker는 connected 상태 유지.
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !srv.broker.IsAGVConnected() {
			t.Fatalf("pong이 정상 송신됐는데 broker가 disconnect로 전이")
		}
		time.Sleep(20 * time.Millisecond)
	}

	<-done
}
