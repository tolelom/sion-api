package services

import (
	"sion-backend/models"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Start/Stop을 반복하며 동시에 Snapshot/IsRunning을 호출해
// data race 없이 종료되는지 검증한다.
// `go test -race` 동작 시 race detector가 문제를 잡아낸다.
func TestSimulatorStartStopUnderLoad(t *testing.T) {
	var broadcasts atomic.Int64
	sim := NewAGVSimulator(func(_ models.WebSocketMessage) {
		broadcasts.Add(1)
	})
	sim.UpdateInterval = 5 * time.Millisecond // 빠른 틱

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 동시 Snapshot 리더
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _, _, _ = sim.Snapshot()
					_ = sim.IsRunning()
				}
			}
		}()
	}

	// Start/Stop 반복
	for i := 0; i < 5; i++ {
		sim.Start()
		time.Sleep(20 * time.Millisecond)
		sim.Stop()
	}

	// 이미 정지된 상태에서 Stop 또 호출 — 블락 없이 즉시 반환되어야 함
	sim.Stop()
	// 중지된 상태에서 Start — 정상 시작 후 즉시 정지 가능해야 함
	sim.Start()
	sim.Stop()

	close(stop)
	wg.Wait()

	if sim.IsRunning() {
		t.Fatal("최종 상태는 stopped여야 함")
	}
}
