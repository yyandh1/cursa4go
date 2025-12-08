package models

import (
	"time"

	"gorm.io/gorm"
)

// ✅ МОДЕЛЬ ПОЛЬЗОВАТЕЛЯ
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"unique;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"` // Не отправлять в JSON
	Role      string         `gorm:"default:'user'" json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Tasks     []Task         `gorm:"foreignKey:UserID" json:"tasks,omitempty"`
	Groups    []Group        `gorm:"many2many:user_groups;" json:"groups,omitempty"`
}

// ✅ МОДЕЛЬ ЗАДАЧИ
type Task struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Title          string     `gorm:"not null" json:"title"`
	Description    string     `json:"description"`
	Status         string     `gorm:"default:'todo'" json:"status"` // todo, in_progress, done
	UserID         *uint      `json:"user_id"`                       // ← УКАЗАТЕЛЬ (может быть NULL)
	GroupID        *uint      `json:"group_id"`                      // ← УКАЗАТЕЛЬ (может быть NULL)
	CreatedByAdmin bool       `gorm:"default:false" json:"created_by_admin"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	User           *User      `gorm:"foreignKey:UserID" json:"-"`
	Group          *Group     `gorm:"foreignKey:GroupID" json:"-"`
	Notes          []Note     `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"notes,omitempty"`

}
// ✅ МОДЕЛЬ ЗАМЕТКИ
type Note struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Content   string         `gorm:"not null" json:"content"`
	TaskID    uint           `json:"task_id"`
	UserID    uint           `json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ✅ МОДЕЛЬ ГРУППЫ (ДОБАВЛЕНО ПОЛЕ DESCRIPTION)
type Group struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"unique;not null" json:"name"`
	Description string         `json:"description"` // ← 🆕 НОВОЕ ПОЛЕ!
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Users       []User         `gorm:"many2many:user_groups;" json:"users,omitempty"`
}
