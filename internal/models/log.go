package models

import (
	"time"
)

// UserLog хранит историю действий пользователя
type UserLog struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"index"`
	Action    string    `json:"action"`  // "login", "lesson_view", "quiz_attempt"
	Details   string    `json:"details"` // Например: "Урок 5", "Вход через Google"
	CreatedAt time.Time `json:"created_at"`

	User User `gorm:"foreignKey:UserID"`
}
