package models

type User struct {
	ID       uint   `gorm:"primaryKey"`
	PublicID string `gorm:"uniqueIndex;size:36"`
	GoogleID string `json:"id"`
	Email    string `gorm:"uniqueIndex;size:255"`
	Name     string
	Picture  string
	RoleID   uint
	Role     Role   `gorm:"foreignKey:RoleID"`
	Language string `gorm:"size:5;default:'ru'"`
}
