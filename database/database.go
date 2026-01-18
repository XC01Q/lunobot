package database

import (
	"database/sql"
	"lunobot/models"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dataSourceName string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dataSourceName+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.init(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER UNIQUE NOT NULL,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			rights INTEGER DEFAULT 1 CHECK (rights >= 1 AND rights <= 3),
			notifications_enabled BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ideas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT,
			content TEXT NOT NULL CHECK (length(content) > 0 AND length(content) <= 4000),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(telegram_id)
		)`,
		`CREATE TABLE IF NOT EXISTS status (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			is_open BOOLEAN DEFAULT TRUE,
			technical_status BOOLEAN DEFAULT TRUE,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_by TEXT DEFAULT 'system'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_ideas_user_id ON ideas(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ideas_created_at ON ideas(created_at DESC)`,
		`INSERT OR IGNORE INTO status (id, is_open, technical_status) VALUES (1, TRUE, TRUE)`,
		`ALTER TABLE users ADD COLUMN notifications_enabled BOOLEAN DEFAULT FALSE`,
		`ALTER TABLE users ADD COLUMN language TEXT DEFAULT 'ua'`,
		`CREATE TABLE IF NOT EXISTS auto_close_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			enabled BOOLEAN DEFAULT FALSE,
			close_time TEXT DEFAULT '22:00',
			keys_to_lobby BOOLEAN DEFAULT TRUE,
			last_status_by TEXT DEFAULT 'system'
		)`,
		`INSERT OR IGNORE INTO auto_close_settings (id) VALUES (1)`,
	}

	for _, query := range queries {
		if _, err := db.conn.Exec(query); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}

	return nil
}

func (db *DB) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, telegram_id, username, first_name, last_name, rights, 
			  COALESCE(language, 'ua') as language,
			  COALESCE(notifications_enabled, 0) as notifications_enabled, created_at, updated_at 
			  FROM users WHERE telegram_id = ?`

	err := db.conn.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.LastName, &user.Rights, &user.Language, &user.NotificationsEnabled, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrUserNotFound
	}

	return user, err
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	username = strings.TrimPrefix(username, "@")

	query := `SELECT id, telegram_id, username, first_name, last_name, rights, 
			  COALESCE(language, 'ua') as language,
			  COALESCE(notifications_enabled, 0) as notifications_enabled, created_at, updated_at 
			  FROM users WHERE username = ? COLLATE NOCASE`

	err := db.conn.QueryRow(query, username).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.LastName, &user.Rights, &user.Language, &user.NotificationsEnabled, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrUserNotFound
	}

	return user, err
}

func (db *DB) CreateUser(user *models.User) error {
	query := `INSERT INTO users (telegram_id, username, first_name, last_name, rights, language, notifications_enabled, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if user.Language == "" {
		user.Language = "ua"
	}
	_, err := db.conn.Exec(query, user.TelegramID, user.Username,
		user.FirstName, user.LastName, user.Rights, user.Language, user.NotificationsEnabled, now, now)

	return err
}

func (db *DB) UpdateUser(user *models.User) error {
	query := `UPDATE users SET username = ?, first_name = ?, last_name = ?, updated_at = ? 
			  WHERE telegram_id = ?`

	_, err := db.conn.Exec(query, user.Username, user.FirstName,
		user.LastName, time.Now(), user.TelegramID)

	return err
}

func (db *DB) UpdateUserRights(telegramID int64, rights models.Rights) error {
	if !rights.IsValid() {
		return models.ErrInvalidRights
	}

	query := `UPDATE users SET rights = ?, updated_at = ? WHERE telegram_id = ?`
	result, err := db.conn.Exec(query, rights, time.Now(), telegramID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

func (db *DB) UpdateUserRightsByUsername(username string, rights models.Rights) error {
	if !rights.IsValid() {
		return models.ErrInvalidRights
	}

	username = strings.TrimPrefix(username, "@")

	query := `UPDATE users SET rights = ?, updated_at = ? WHERE username = ? COLLATE NOCASE`
	result, err := db.conn.Exec(query, rights, time.Now(), username)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

func (db *DB) UpdateNotificationsEnabled(telegramID int64, enabled bool) error {
	query := `UPDATE users SET notifications_enabled = ?, updated_at = ? WHERE telegram_id = ?`
	result, err := db.conn.Exec(query, enabled, time.Now(), telegramID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

func (db *DB) UpdateUserLanguage(telegramID int64, language string) error {
	query := `UPDATE users SET language = ?, updated_at = ? WHERE telegram_id = ?`
	result, err := db.conn.Exec(query, language, time.Now(), telegramID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

func (db *DB) GetAllAdmins() ([]models.User, error) {
	query := `SELECT id, telegram_id, username, first_name, last_name, rights, 
			  COALESCE(language, 'ua') as language,
			  COALESCE(notifications_enabled, 0) as notifications_enabled, created_at, updated_at
              FROM users WHERE rights >= ?`
	rows, err := db.conn.Query(query, models.RightsManager)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
			&user.LastName, &user.Rights, &user.Language, &user.NotificationsEnabled, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (db *DB) GetUsersWithNotifications() ([]models.User, error) {
	query := `SELECT id, telegram_id, username, first_name, last_name, rights, 
			  COALESCE(language, 'ua') as language,
			  COALESCE(notifications_enabled, 0) as notifications_enabled, created_at, updated_at
              FROM users WHERE notifications_enabled = 1`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
			&user.LastName, &user.Rights, &user.Language, &user.NotificationsEnabled, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (db *DB) AddIdea(idea *models.Idea) error {
	if err := idea.Validate(); err != nil {
		return err
	}

	query := `INSERT INTO ideas (user_id, username, content) VALUES (?, ?, ?)`
	_, err := db.conn.Exec(query, idea.UserID, idea.Username, idea.Content)
	return err
}

func (db *DB) GetIdeaByID(ideaID int64) (*models.Idea, error) {
	idea := &models.Idea{}
	query := `SELECT id, user_id, username, content, created_at FROM ideas WHERE id = ?`

	err := db.conn.QueryRow(query, ideaID).Scan(
		&idea.ID, &idea.UserID, &idea.Username, &idea.Content, &idea.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrIdeaNotFound
	}

	return idea, err
}

func (db *DB) DeleteIdea(ideaID int64) error {
	query := `DELETE FROM ideas WHERE id = ?`
	result, err := db.conn.Exec(query, ideaID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrIdeaNotFound
	}

	return nil
}

func (db *DB) GetAllIdeas() ([]models.Idea, error) {
	query := `SELECT id, user_id, username, content, created_at FROM ideas ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []models.Idea
	for rows.Next() {
		var idea models.Idea
		err := rows.Scan(&idea.ID, &idea.UserID, &idea.Username, &idea.Content, &idea.CreatedAt)
		if err != nil {
			return nil, err
		}
		ideas = append(ideas, idea)
	}

	return ideas, nil
}

func (db *DB) GetStatus() (*models.Status, error) {
	status := &models.Status{}
	query := `SELECT id, is_open, technical_status, updated_at, updated_by FROM status WHERE id = 1`

	err := db.conn.QueryRow(query).Scan(
		&status.ID, &status.IsOpen, &status.TechnicalStatus, &status.UpdatedAt, &status.UpdatedBy,
	)

	return status, err
}

func (db *DB) UpdateOpenStatus(isOpen bool, user *models.User) error {
	query := `UPDATE status SET is_open = ?, technical_status = ?, updated_at = ?, updated_by = ? WHERE id = 1`
	_, err := db.conn.Exec(query, isOpen, isOpen, time.Now(), user.GetDisplayName())
	return err
}

func (db *DB) UpdateTechnicalStatus(technicalStatus bool, user *models.User) error {
	query := `UPDATE status SET technical_status = ?, updated_at = ?, updated_by = ? WHERE id = 1`
	_, err := db.conn.Exec(query, technicalStatus, time.Now(), user.GetDisplayName())
	return err
}

func (db *DB) GetAutoCloseSettings() (*models.AutoCloseSettings, error) {
	settings := &models.AutoCloseSettings{}
	query := `SELECT id, enabled, close_time, keys_to_lobby, last_status_by FROM auto_close_settings WHERE id = 1`

	err := db.conn.QueryRow(query).Scan(
		&settings.ID, &settings.Enabled, &settings.CloseTime, &settings.KeysToLobby, &settings.LastStatusBy,
	)

	return settings, err
}

func (db *DB) UpdateAutoCloseSettings(enabled bool, closeTime string, keysToLobby bool) error {
	query := `UPDATE auto_close_settings SET enabled = ?, close_time = ?, keys_to_lobby = ? WHERE id = 1`
	_, err := db.conn.Exec(query, enabled, closeTime, keysToLobby)
	return err
}

func (db *DB) UpdateAutoCloseLastUser(username string) error {
	query := `UPDATE auto_close_settings SET last_status_by = ? WHERE id = 1`
	_, err := db.conn.Exec(query, username)
	return err
}

func (db *DB) UpdateOpenStatusAuto(isOpen bool, keysToLobby bool, updatedBy string) error {
	query := `UPDATE status SET is_open = ?, technical_status = ?, updated_at = ?, updated_by = ? WHERE id = 1`
	_, err := db.conn.Exec(query, isOpen, !keysToLobby, time.Now(), updatedBy)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}
