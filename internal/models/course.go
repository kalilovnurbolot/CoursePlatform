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

	Title       string `json:"title"`
	Description string `json:"description"`
	IsPublished bool   `json:"is_published"`
	AuthorID    uint   `json:"author_id"`

	Author  User     `json:"author" gorm:"foreignKey:AuthorID"`
	Modules []Module `json:"modules" gorm:"constraint:OnDelete:CASCADE;"` // Удаление курса удалит модули
}

// Module (Модуль)
type Module struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Title    string   `json:"title"`
	CourseID uint     `json:"course_id"`
	Lessons  []Lesson `json:"lessons" gorm:"constraint:OnDelete:CASCADE;"` // Удаление модуля удалит уроки
}

// Lesson (Урок)
type Lesson struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Title    string `json:"title"`
	ModuleID uint   `json:"module_id"`

	// Связь: Один урок имеет много блоков контента
	// При удалении урока удалятся и блоки (CASCADE)
	ContentBlocks []ContentBlock `json:"content_blocks" gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE;"`
}

// ContentBlock (Таблица контента)
type ContentBlock struct {
	ID       uint `gorm:"primarykey" json:"id"`
	LessonID uint `json:"lesson_id"` // Внешний ключ

	Type  string `json:"type"`  // "text", "code", "video", "quiz"
	Order int    `json:"order"` // Порядок отображения (0, 1, 2...)

	// JSONB поле. Хранит { "content": "...", "extra": "..." }
	// Если используете SQLite, можно заменить на просто string (text)
	Data datatypes.JSON `json:"data"`
}
