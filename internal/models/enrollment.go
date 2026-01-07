package models

import "gorm.io/gorm"

// Enrollment (Заявка на курс / Подписка)
type Enrollment struct {
	gorm.Model
	UserID   uint   `json:"user_id"`
	CourseID uint   `json:"course_id"`
	Status   string `json:"status"` // pending, approved, rejected

	// Убираем json:"-" чтобы видеть данные в API
	User   User   `json:"user" gorm:"foreignKey:UserID"`
	Course Course `json:"course" gorm:"foreignKey:CourseID"`
}
