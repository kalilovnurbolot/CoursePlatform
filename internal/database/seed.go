package database

import (
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	roles := []models.Role{
		{ID: 1, Name: "Student"}, // В логах твое приложение ждет ID: 1
		{ID: 2, Name: "Teacher"},
		{ID: 3, Name: "Admin"},
	}

	for _, role := range roles {
		// Ищем роль по ID, если не нашли — создаем с указанным Name
		err := db.Where(models.Role{ID: role.ID}).FirstOrCreate(&role).Error
		if err != nil {
			return err
		}
	}
	return nil
}
