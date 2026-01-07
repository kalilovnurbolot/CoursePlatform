package database

import (
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Course{},
		&models.Module{},
		&models.Lesson{},
		&models.ContentBlock{},
		&models.Enrollment{},
	)
}
