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

type LessonProgress struct {
	ID uint `gorm:"primarykey" json:"id"`
	// Составной индекс: поиск прогресса конкретного юзера по конкретному уроку будет мгновенным
	UserID   uint `json:"user_id" gorm:"uniqueIndex:idx_user_lesson"`
	LessonID uint `json:"lesson_id" gorm:"uniqueIndex:idx_user_lesson"`

	CourseID  uint      `json:"course_id" gorm:"index"` // Индекс для быстрого счета прогресса всего курса
	IsDone    bool      `json:"is_done" gorm:"default:false"`
	UpdatedAt time.Time `json:"completed_at"`
}
