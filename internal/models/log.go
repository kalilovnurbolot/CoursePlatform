package models

import (
	"time"
)

const (
	LogLogin           = "login"
	LogLessonView      = "lesson_view"
	LogQuizAttempt     = "quiz_attempt"
	LogCourseComplete  = "course_complete"
	LogReviewAdded     = "review_added"
)

// UserLog хранит историю действий пользователя
type UserLog struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"index"`
	Action    string    `json:"action"`   // см. константы Log*
	Details   string    `json:"details"`  // текстовый контекст: "Урок 5", "Курс Go basics"
	CourseID  uint      `gorm:"index" json:"course_id"`  // 0 если не применимо
	LessonID  uint      `gorm:"index" json:"lesson_id"`  // 0 если не применимо
	CreatedAt time.Time `json:"created_at"`

	User User `gorm:"foreignKey:UserID"`
}
