package database

import (
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Course{},
		&models.Module{},
		&models.Lesson{},
		&models.ContentBlock{},
		&models.Enrollment{},
		&models.LessonProgress{},
		&models.QuizAttempt{},
		&models.Comment{},
		&models.Review{},
		&models.Certificate{},
		&models.UserLog{},
		&models.Reaction{},
	); err != nil {
		return err
	}

	// Backfill public_id for existing users that don't have one yet,
	// then enforce NOT NULL — safe to run on every startup (idempotent).
	if err := db.Exec(`
		UPDATE users SET public_id = gen_random_uuid()::varchar WHERE public_id IS NULL OR public_id = ''
	`).Error; err != nil {
		return err
	}
	return db.Exec(`ALTER TABLE users ALTER COLUMN public_id SET NOT NULL`).Error
}
