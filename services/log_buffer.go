package services

import (
	"log"
	"sion-backend/models"
	"sync"
	"time"
)

// LogConfig는 로깅 버퍼·재시도 정책을 한 곳에 모은다.
// 값이 0 이하인 필드는 InitLogging에서 기본값으로 대체된다.
type LogConfig struct {
	FlushSize     int           // 메모리 버퍼가 이 크기에 도달하면 즉시 Flush 트리거
	FlushInterval time.Duration // autoFlush 주기
	MaxRetries    int           // 실패 로그 재시도 횟수 한도 (초과 시 폐기)
	MaxFailedLogs int           // 실패 큐 상한 (초과 시 오래된 것부터 폐기)
}

const (
	defaultFlushSize     = 50
	defaultFlushInterval = 10 * time.Second
	defaultMaxRetries    = 3
	defaultMaxFailedLogs = 500
)

type retryableLog struct {
	Log        models.AGVLog
	RetryCount int
}

type LogBuffer struct {
	logs          []models.AGVLog
	failedLogs    []retryableLog
	mu            sync.Mutex
	flushSize     int
	flushTime     time.Duration
	maxRetries    int
	maxFailedLogs int
	stopChan      chan struct{}
	doneChan      chan struct{}
}

var logBuffer *LogBuffer

func InitLogging(cfg LogConfig) {
	if cfg.FlushSize <= 0 {
		cfg.FlushSize = defaultFlushSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.MaxFailedLogs <= 0 {
		cfg.MaxFailedLogs = defaultMaxFailedLogs
	}

	logBuffer = &LogBuffer{
		logs:          make([]models.AGVLog, 0, cfg.FlushSize*2),
		failedLogs:    make([]retryableLog, 0),
		flushSize:     cfg.FlushSize,
		flushTime:     cfg.FlushInterval,
		maxRetries:    cfg.MaxRetries,
		maxFailedLogs: cfg.MaxFailedLogs,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
	}

	go logBuffer.autoFlush()

	log.Printf("[INFO] 로깅 시스템 초기화 (flushSize=%d, interval=%v, maxRetries=%d, maxFailedLogs=%d)",
		cfg.FlushSize, cfg.FlushInterval, cfg.MaxRetries, cfg.MaxFailedLogs)
}

// StopLogging은 autoFlush 고루틴의 마지막 Flush가 끝날 때까지 동기 대기한다.
// 이전 구현은 stopChan 송신 직후 리턴해서 프로세스 종료 시 버퍼/재시도 큐가 유실됐다.
func StopLogging() {
	if logBuffer == nil {
		return
	}
	close(logBuffer.stopChan)
	<-logBuffer.doneChan
	log.Println("[INFO] 로깅 시스템 종료")
}

func (lb *LogBuffer) autoFlush() {
	defer close(lb.doneChan)
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
				if rl.RetryCount >= lb.maxRetries {
					log.Printf("[WARN] 로그 재시도 %d회 초과, 폐기", lb.maxRetries)
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
		if len(lb.failedLogs) > lb.maxFailedLogs {
			dropped := len(lb.failedLogs) - lb.maxFailedLogs
			lb.failedLogs = lb.failedLogs[dropped:]
			log.Printf("[WARN] 실패 로그 큐 초과, %d개 폐기", dropped)
		}
		lb.mu.Unlock()
	}
}
