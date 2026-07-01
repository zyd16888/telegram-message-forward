package database

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"log"
	"time"
)

func InitDB(dbPath string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	s, err := db.DB()
	if err != nil {
		log.Fatalf("open database fail: %v", err)
	}

	s.SetConnMaxIdleTime(10)
	s.SetMaxOpenConns(100)
	s.SetConnMaxIdleTime(1 * time.Hour)

	return db
}
