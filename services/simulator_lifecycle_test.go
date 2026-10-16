package services

import (
	"sion-backend/models"
	"sync"
	"testing"
	"time"
)

func TestSimulator_ConcurrentStartStopIsSafe(t *testing.T) {
	sim := NewAGVSimulator(func(models.WebSocketMessage) {})
	sim.UpdateInterval = time.Millisecond
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sim.Start()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sim.Stop()
			}
		}()
	}
	wg.Wait()
	sim.Stop()
	if sim.IsRunning() {
		t.Fatal("simulator still running after final Stop")
	}
}

func TestSimulator_SlowBroadcastDoesNotBlockSnapshot(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	entered := make(chan struct{})
	sim := NewAGVSimulator(func(models.WebSocketMessage) {
		once.Do(func() { close(entered) })
		<-release
	})
	sim.UpdateInterval = time.Millisecond
	sim.Start()
	defer func() {
		close(release)
		sim.Stop()
	}()
	<-entered

	got := make(chan struct{})
	go func() {
		sim.Snapshot()
		close(got)
	}()
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("Snapshot blocked while a broadcast was in progress")
	}
}
