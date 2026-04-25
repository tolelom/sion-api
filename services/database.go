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

func InitDatabase() error {
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
