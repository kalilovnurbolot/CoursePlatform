package models

import (
	"time"

	"gorm.io/gorm"
)

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

// models/progress.go

type LessonProgress struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_lesson" json:"user_id"`
	LessonID  uint      `gorm:"uniqueIndex:idx_user_lesson" json:"lesson_id"`
	CourseID  uint      `gorm:"index" json:"course_id"`
	IsDone    bool      `json:"is_done" gorm:"default:false"`
	UpdatedAt time.Time `json:"completed_at"`
}

// Новая модель для записи результатов тестов
type QuizAttempt struct {
	ID            uint      `gorm:"primarykey"`
	UserID        uint      `gorm:"index"`
	LessonID      uint      `gorm:"index"`
	BlockID       uint      `json:"block_id" gorm:"index"` // Прямая связь с ID блока
	Question      string    `json:"question"`
	Answer        string    `json:"answer"`
	IsCorrect     bool      `json:"is_correct"`
	SelectedIndex int       `json:"selected_index"` // Сохраняем номер ответа (0, 1, 2...)
	CreatedAt     time.Time `json:"created_at"`
}
