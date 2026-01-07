package database

import (
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	db.FirstOrCreate(&models.Role{}, models.Role{ID: 1, Name: "User"})
	db.FirstOrCreate(&models.Role{}, models.Role{ID: 2, Name: "Admin"})
	return nil
}
