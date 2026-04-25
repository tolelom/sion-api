package services

import (
	"log"
	"sion-backend/models"
	"sync"
	"time"
)

const (
	maxRetries    = 3
	maxFailedLogs = 500
)

type retryableLog struct {
	Log        models.AGVLog
	RetryCount int
}

type LogBuffer struct {
	logs       []models.AGVLog
	failedLogs []retryableLog
	mu         sync.Mutex
	flushSize  int
	flushTime  time.Duration
	stopChan   chan bool
}

var logBuffer *LogBuffer

func InitLogging(flushSize int, flushInterval time.Duration) {
	logBuffer = &LogBuffer{
		logs:       make([]models.AGVLog, 0, flushSize*2),
		failedLogs: make([]retryableLog, 0),
		flushSize:  flushSize,
		flushTime:  flushInterval,
		stopChan:   make(chan bool),
	}

	go logBuffer.autoFlush()

	log.Printf("[INFO] 로깅 시스템 초기화 (flushSize=%d, interval=%v)", flushSize, flushInterval)
}

func StopLogging() {
	if logBuffer != nil {
		logBuffer.stopChan <- true
		log.Println("[INFO] 로깅 시스템 종료")
	}
}

func (lb *LogBuffer) autoFlush() {
	ticker := time.NewTicker(lb.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.Flush()
		case <-lb.stopChan:
			lb.Flush()
			return
		}
	}
}

func AddLog(logEntry models.AGVLog) {
	if logBuffer == nil {
		log.Println("[WARN] 로깅 시스템이 초기화되지 않음")
		return
	}

	logBuffer.mu.Lock()
	logBuffer.logs = append(logBuffer.logs, logEntry)
	size := len(logBuffer.logs)
	logBuffer.mu.Unlock()

	if size >= logBuffer.flushSize {
		go logBuffer.Flush()
	}
}

func (lb *LogBuffer) Flush() {
	if db == nil {
		return
	}

	lb.mu.Lock()
	if len(lb.logs) == 0 && len(lb.failedLogs) == 0 {
		lb.mu.Unlock()
		return
	}

	var retryLogs []retryableLog
	if len(lb.failedLogs) > 0 {
		retryLogs = make([]retryableLog, len(lb.failedLogs))
		copy(retryLogs, lb.failedLogs)
		lb.failedLogs = lb.failedLogs[:0]
	}

	var newLogs []models.AGVLog
	if len(lb.logs) > 0 {
		newLogs = make([]models.AGVLog, len(lb.logs))
		copy(newLogs, lb.logs)
		lb.logs = lb.logs[:0]
	}
	lb.mu.Unlock()

	var stillFailed []retryableLog
	if len(retryLogs) > 0 {
		retryBatch := make([]models.AGVLog, len(retryLogs))
		for i, rl := range retryLogs {
			retryBatch[i] = rl.Log
		}
		if err := db.CreateInBatches(retryBatch, 100).Error; err != nil {
			log.Printf("[ERROR] 재시도 로그 저장 실패: %v", err)
			for _, rl := range retryLogs {
				rl.RetryCount++
				if rl.RetryCount >= maxRetries {
					log.Printf("[WARN] 로그 재시도 %d회 초과, 폐기", maxRetries)
				} else {
					stillFailed = append(stillFailed, rl)
				}
			}
		} else {
			log.Printf("[INFO] 재시도 로그 %d개 저장 완료", len(retryBatch))
		}
	}

	if len(newLogs) > 0 {
		if err := db.CreateInBatches(newLogs, 100).Error; err != nil {
			log.Printf("[ERROR] 로그 저장 실패: %v", err)
			for _, l := range newLogs {
				stillFailed = append(stillFailed, retryableLog{Log: l, RetryCount: 0})
			}
		} else {
			log.Printf("[INFO] 로그 %d개 저장 완료", len(newLogs))
		}
	}

	if len(stillFailed) > 0 {
		lb.mu.Lock()
		lb.failedLogs = append(lb.failedLogs, stillFailed...)
		if len(lb.failedLogs) > maxFailedLogs {
			dropped := len(lb.failedLogs) - maxFailedLogs
			lb.failedLogs = lb.failedLogs[dropped:]
			log.Printf("[WARN] 실패 로그 큐 초과, %d개 폐기", dropped)
		}
		lb.mu.Unlock()
	}
}
