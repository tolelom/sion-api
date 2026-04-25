package services

import (
	"sion-backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetTestDB는 테스트 코드 전용으로 패키지 db 변수를 교체한다.
// (테스트가 끝난 뒤 nil로 복원해 격리 보장)
func SetTestDB(database *gorm.DB) {
	db = database
}

// NewInMemoryDB는 인메모리 sqlite GORM DB를 생성하고 AGVLog 스키마를 마이그레이션한다.
// 테스트에서 호출해 production MySQL 의존을 우회한다.
func NewInMemoryDB() (*gorm.DB, error) {
	// shared cache로 같은 DSN을 재사용하면 같은 인스턴스를 공유. 테스트마다 격리하려면 file::memory:?cache=shared 대신 :memory: 사용.
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := gdb.AutoMigrate(&models.AGVLog{}); err != nil {
		return nil, err
	}
	return gdb, nil
}
