package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	// Берем строку подключения из .env через переменную окружения
	dsn := os.Getenv("DATABASE_URL")

	// Если переменная пустая (например, забыли пробросить .env),
	// используем локальный дефолт для подстраховки
	if dsn == "" {
		dsn = "host=db user=postgres password=1234 dbname=testdb port=5432 sslmode=disable"
	}

	var db *gorm.DB
	var err error

	// Попытки подключения (Docker-база иногда «просыпается» пару секунд)
	for i := 0; i < 5; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			log.Println("✅ Успешное подключение к базе данных!")
			return db, nil
		}

		log.Printf("⚠️ Попытка подключения %d не удалась, ждем... (%v)", i+1, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("не удалось подключиться к БД после нескольких попыток: %w", err)
}
