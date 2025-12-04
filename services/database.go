package services

import (
	"fmt"
	"log"
	"os"
	"sion-backend/models"
	"strconv"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 인스턴스
var db *gorm.DB

// InitDatabase - 환경 변수로 MySQL 연결
func InitDatabase() error {
	// 환경 변수에서 DSN 구성
	host := os.Getenv("MYSQL_HOST")
	portStr := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	dbname := os.Getenv("MYSQL_DATABASE")

	if host == "" || user == "" || password == "" || dbname == "" {
		return fmt.Errorf("MySQL 환경 변수가 모두 설정되지 않았습니다: MYSQL_HOST, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		port = 3306 // 기본 포트
	}

	// DSN 구성
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	var errDB error
	db, errDB = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if errDB != nil {
		return fmt.Errorf("DB 연결 실패: %v", errDB)
	}

	// AutoMigrate - 테이블 자동 생성
	errMigrate := db.AutoMigrate(
		&models.AGVLog{},
	)
	if errMigrate != nil {
		return fmt.Errorf("마이그레이션 실패: %v", errMigrate)
	}

	log.Println("✅ MySQL 연결 및 마이그레이션 완료")
	log.Printf("📡 연결 정보: %s:%s@%s:%d/%s", user, password[:3]+"***", host, port, dbname)
	return nil
}

// GetDB - GORM 인스턴스 반환
func GetDB() *gorm.DB {
	return db
}

// 🆕 LogAGVEvent - AGV 이벤트 로깅 (호환성 유지용)
// 새로운 logging.go의 함수를 사용하는 것을 권장합니다.
func LogAGVEvent(msg models.WebSocketMessage, agvID string, userID string) error {
	// 새로운 logging 서비스 사용
	LogWebSocketMessage(agvID, msg)
	return nil
}

// GetRecentLogs - 최근 로그 조회 (LLM 컨텍스트용)
func GetRecentLogs(agvID string, limit int) ([]models.AGVLog, error) {
	var logs []models.AGVLog
	err := db.Where("agv_id = ?", agvID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// marshalMessageData - 메시지 데이터 JSON 직렬화 (간단 구현)
func marshalMessageData(data interface{}) string {
	return fmt.Sprintf("%v", data)
}
