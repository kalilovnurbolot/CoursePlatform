package models

type User struct {
	ID       uint   `gorm:"primaryKey"`
	GoogleID string `json:"id"`
	Email    string `gorm:"uniqueIndex;size:255"`
	Name     string
	Picture  string
	RoleID   uint
	Role     Role `gorm:"foreignKey:RoleID"`
}
