package models

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrIdeaNotFound  = errors.New("idea not found")
	ErrInvalidRights = errors.New("invalid rights level")
	ErrDuplicateUser = errors.New("user already exists")
)

type User struct {
	ID                   int64     `json:"id" db:"id"`
	TelegramID           int64     `json:"telegram_id" db:"telegram_id"`
	Username             string    `json:"username" db:"username"`
	FirstName            string    `json:"first_name" db:"first_name"`
	LastName             string    `json:"last_name" db:"last_name"`
	Rights               Rights    `json:"rights" db:"rights"`
	Language             string    `json:"language" db:"language"`
	NotificationsEnabled bool      `json:"notifications_enabled" db:"notifications_enabled"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

func (u *User) GetDisplayName() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	name := u.FirstName
	if u.LastName != "" {
		name += " " + u.LastName
	}
	return name
}

func (u *User) HasRights(required Rights) bool {
	return u.Rights >= required
}

type Idea struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Username  string    `json:"username" db:"username"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func (i *Idea) Validate() error {
	if i.Content == "" {
		return errors.New("idea content cannot be empty")
	}
	if len(i.Content) > 4000 {
		return errors.New("idea content too long")
	}
	return nil
}

type Status struct {
	ID              int       `json:"id" db:"id"`
	IsOpen          bool      `json:"is_open" db:"is_open"`
	TechnicalStatus bool      `json:"technical_status" db:"technical_status"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy       string    `json:"updated_by" db:"updated_by"`
}

type Rights int

const (
	RightsDefault Rights = 1
	RightsManager Rights = 2
	RightsAdmin   Rights = 3
)

func (r Rights) IsValid() bool {
	return r >= RightsDefault && r <= RightsAdmin
}

func (r Rights) String() string {
	switch r {
	case RightsDefault:
		return "Юзер"
	case RightsManager:
		return "Адмін"
	case RightsAdmin:
		return "Бос"
	default:
		return "Невідомий"
	}
}

type AutoCloseSettings struct {
	ID           int    `json:"id" db:"id"`
	Enabled      bool   `json:"enabled" db:"enabled"`
	CloseTime    string `json:"close_time" db:"close_time"`
	KeysToLobby  bool   `json:"keys_to_lobby" db:"keys_to_lobby"`
	LastStatusBy string `json:"last_status_by" db:"last_status_by"`
}
