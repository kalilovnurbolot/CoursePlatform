package models

import "time"

type Reaction struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	UserID     uint      `gorm:"uniqueIndex:idx_reaction;not null" json:"user_id"`
	TargetType string    `gorm:"uniqueIndex:idx_reaction;not null" json:"target_type"` // "course" | "lesson"
	TargetID   uint      `gorm:"uniqueIndex:idx_reaction;not null" json:"target_id"`
	Type       string    `gorm:"not null" json:"type"` // "like" | "dislike"

	User User `json:"user" gorm:"foreignKey:UserID"`
}
