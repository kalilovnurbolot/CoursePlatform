package storage

import (
	"errors"

	"github.com/s/onlineCourse/internal/models"
	"gorm.io/gorm"
)

// internal/storage/user.go

// SaveUser finds a user by Google ID or email; if found, it updates, otherwise, it creates.
func SaveUser(db *gorm.DB, userInfo models.User) (uint, error) {
	var existingUser models.User

	// 1. Attempt to find the user based on their Google ID
	// Note: If you rely only on email, it's possible for a user to sign up
	// with a password (if you had that) and then try to sign in with Google OAuth later,
	// causing a conflict if the Google ID isn't linked.

	result := db.Where("google_id = ?", userInfo.GoogleID).First(&existingUser)

	if result.Error == nil {
		// --- CASE 1: USER FOUND (UPDATE) ---
		// User exists, update their details (name, picture, etc.)
		updates := map[string]interface{}{
			"email":   userInfo.Email, // Email should already be correct but update just in case
			"name":    userInfo.Name,
			"picture": userInfo.Picture,
			// DO NOT update RoleID here, as that is managed by an admin
		}

		db.Model(&existingUser).Updates(updates)
		return existingUser.ID, nil

	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// --- CASE 2: USER NOT FOUND (CREATE) ---
		// User is new. Set their default role ID (e.g., RoleUser = 1)
		userInfo.RoleID = models.RoleUser

		// Use GORM's Create method (this is where the original INSERT failed)
		if err := db.Create(&userInfo).Error; err != nil {
			return 0, err
		}
		return userInfo.ID, nil

	} else {
		// --- CASE 3: DATABASE ERROR ---
		return 0, result.Error
	}
}
