package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Course (Курс)
type Course struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	ImageURL  string         `json:"image_url"` // <--- Restored
	Language  string         `json:"language"`  // <--- Restored

	Title       string `json:"title"`
	Description string `json:"description"`
	IsPublished bool   `json:"is_published"`
	AuthorID    uint   `json:"author_id"`

	Author  User     `json:"author" gorm:"foreignKey:AuthorID"`
	Modules []Module `json:"modules" gorm:"constraint:OnDelete:CASCADE;"`
}

// Module (Модуль)
type Module struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Title    string   `json:"title"`
	CourseID uint     `json:"course_id"`
	Lessons  []Lesson `json:"lessons" gorm:"constraint:OnDelete:CASCADE;"`
}

// Lesson (Урок)
type Lesson struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Title    string `json:"title"`
	ModuleID uint   `json:"module_id"`

	ContentBlocks []ContentBlock `json:"content_blocks" gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE;"`
}

// ContentBlock (Таблица контента)
type ContentBlock struct {
	ID       uint `gorm:"primarykey" json:"id"`
	LessonID uint `json:"lesson_id"`

	Type  string `json:"type"`  // "text", "code", "video", "quiz", "vocabulary", "audio_dictation"
	Order int    `json:"order"`

	Data datatypes.JSON `json:"data"`
}
