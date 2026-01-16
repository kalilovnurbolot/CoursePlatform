package models

import (
	"time"

	"gorm.io/gorm"
)

// Comment - Комментарий к уроку
type Comment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `json:"user_id"`
	LessonID uint   `json:"lesson_id"`
	Content  string `json:"content"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}

// Review - Отзыв к курсу
type Review struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `json:"user_id"`
	CourseID uint   `json:"course_id"`
	Rating   int    `json:"rating"` // 1-5
	Content  string `json:"content"`

	User   User   `json:"user" gorm:"foreignKey:UserID"`
	Course Course `json:"course" gorm:"foreignKey:CourseID"` // <--- Добавлено
}
