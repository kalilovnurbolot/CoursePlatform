package models

type Role struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`

	Users []User
}

// Константы для RoleID, используемые по всему приложению.
// Используем uint, если RoleID в структуре User - uint.
// Если в main вы используете int, то здесь тоже используйте int.
// Я буду использовать int для соответствия вашим константам в main.
// internal/models/roles.go

const (
	// Change type from int to uint to match the database model
	RoleGuest   uint = 0
	RoleUser    uint = 1
	RoleAdmin   uint = 2
	RoleManager uint = 3
)
