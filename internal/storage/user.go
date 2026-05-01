package storage

import (
	"errors"

	"github.com/google/uuid"
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

// SaveUser finds a user by Google ID; if found, updates name/picture, otherwise creates.
func SaveUser(db *gorm.DB, userInfo models.User) (uint, error) {
	var existingUser models.User

	result := db.Where("google_id = ?", userInfo.GoogleID).First(&existingUser)

	if result.Error == nil {
		updates := map[string]interface{}{
			"email":   userInfo.Email,
			"name":    userInfo.Name,
			"picture": userInfo.Picture,
		}
		// back-fill PublicID for users created before this field existed
		if existingUser.PublicID == "" {
			updates["public_id"] = uuid.NewString()
		}
		db.Model(&existingUser).Updates(updates)
		return existingUser.ID, nil

	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		userInfo.RoleID = models.RoleUser
		userInfo.PublicID = uuid.NewString()
		if err := db.Create(&userInfo).Error; err != nil {
			return 0, err
		}
		return userInfo.ID, nil

	} else {
		return 0, result.Error
	}
}
