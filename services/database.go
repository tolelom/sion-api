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

var db *gorm.DB

// InitDatabase는 SION_USE_IN_MEMORY_DB=true이면 pure-Go sqlite 인메모리 DB로,
// 그 외에는 기존 MySQL 환경변수 경로로 초기화한다.
// 인메모리 모드는 dev/E2E 전용이며 프로세스 종료 시 데이터가 사라진다.
// production에서는 env를 절대 켜지 말 것 — 시작 로그에 큰 경고를 남긴다.
func InitDatabase() error {
	if IsTruthyEnv("SION_USE_IN_MEMORY_DB") {
		return initInMemoryDB()
	}
	return initMySQL()
}

// IsTruthyEnv는 opt-in 플래그 환경변수의 해석을 한 곳에 모은다.
// 미설정·오타·"false"는 모두 false — 켜려면 명시적으로 truthy 값을 줘야 한다.
func IsTruthyEnv(key string) bool {
	v := os.Getenv(key)
	switch v {
	case "1", "true", "TRUE", "True", "yes", "YES":
		return true
	}
	return false
}

func initInMemoryDB() error {
	log.Println("[WARN] ⚠ SION_USE_IN_MEMORY_DB=true — 인메모리 SQLite로 시작합니다.")
	log.Println("[WARN] ⚠ 이 모드는 dev/E2E 전용입니다. 프로세스 종료 시 모든 로그가 사라집니다.")

	gdb, err := NewInMemoryDB()
	if err != nil {
		return fmt.Errorf("인메모리 DB 초기화 실패: %w", err)
	}
	db = gdb
	log.Println("[INFO] 인메모리 SQLite 연결 + AGVLog 마이그레이션 완료")
	return nil
}

func initMySQL() error {
	host := os.Getenv("MYSQL_HOST")
	portStr := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	dbname := os.Getenv("MYSQL_DATABASE")

	if host == "" || user == "" || password == "" || dbname == "" {
		return fmt.Errorf("MySQL 환경 변수가 모두 설정되지 않았습니다: MYSQL_HOST, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE (또는 dev/E2E라면 SION_USE_IN_MEMORY_DB=true 설정)")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	var errDB error
	db, errDB = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if errDB != nil {
		return fmt.Errorf("DB 연결 실패: %v", errDB)
	}

	errMigrate := db.AutoMigrate(
		&models.AGVLog{},
	)
	if errMigrate != nil {
		return fmt.Errorf("마이그레이션 실패: %v", errMigrate)
	}

	log.Println("[INFO] MySQL 연결 및 마이그레이션 완료")
	log.Printf("[INFO] DB 연결: %s@%s:%d/%s", user, host, port, dbname)
	return nil
}

func GetDB() *gorm.DB {
	return db
}
