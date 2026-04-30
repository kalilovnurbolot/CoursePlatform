package models

import (
	"time"

	"gorm.io/gorm"
)

type Certificate struct {
	gorm.Model
	UserID   uint      `gorm:"uniqueIndex:idx_user_course_cert;index" json:"user_id"`
	CourseID uint      `gorm:"uniqueIndex:idx_user_course_cert;index" json:"course_id"`
	Code     string    `gorm:"uniqueIndex;size:64" json:"code"` // уникальный хэш для верификации
	IssuedAt time.Time `json:"issued_at"`

	User   User   `json:"user" gorm:"foreignKey:UserID"`
	Course Course `json:"course" gorm:"foreignKey:CourseID"`
}
