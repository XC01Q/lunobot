package services

import (
	"lunobot/database"
	"lunobot/models"
)

type UserService struct {
	db *database.DB
}

func NewUserService(db *database.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetOrCreateUser(telegramID int64, username, firstName, lastName string) (*models.User, error) {
	user, err := s.db.GetUserByTelegramID(telegramID)
	if err == models.ErrUserNotFound {
		user = &models.User{
			TelegramID:           telegramID,
			Username:             username,
			FirstName:            firstName,
			LastName:             lastName,
			Rights:               models.RightsDefault,
			NotificationsEnabled: false,
		}
		if err := s.db.CreateUser(user); err != nil {
			return nil, err
		}
		return user, nil
	} else if err != nil {
		return nil, err
	}

	if user.Username != username || user.FirstName != firstName || user.LastName != lastName {
		user.Username = username
		user.FirstName = firstName
		user.LastName = lastName
		if err := s.db.UpdateUser(user); err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *UserService) UpdateUserRights(telegramID int64, rights models.Rights) error {
	return s.db.UpdateUserRights(telegramID, rights)
}

func (s *UserService) UpdateUserRightsByUsername(username string, rights models.Rights) error {
	return s.db.UpdateUserRightsByUsername(username, rights)
}

func (s *UserService) GetUserByUsername(username string) (*models.User, error) {
	return s.db.GetUserByUsername(username)
}

func (s *UserService) UpdateNotificationsEnabled(telegramID int64, enabled bool) error {
	return s.db.UpdateNotificationsEnabled(telegramID, enabled)
}

func (s *UserService) GetUsersWithNotifications() ([]models.User, error) {
	return s.db.GetUsersWithNotifications()
}

func (s *UserService) GetAllAdmins() ([]models.User, error) {
	return s.db.GetAllAdmins()
}

func (s *UserService) UpdateUserLanguage(telegramID int64, language string) error {
	return s.db.UpdateUserLanguage(telegramID, language)
}
