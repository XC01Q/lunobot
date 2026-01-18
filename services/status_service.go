package services

import (
	"lunobot/database"
	"lunobot/models"
)

type StatusService struct {
	db *database.DB
}

func NewStatusService(db *database.DB) *StatusService {
	return &StatusService{db: db}
}

func (s *StatusService) GetStatus() (*models.Status, error) {
	return s.db.GetStatus()
}

func (s *StatusService) UpdateOpenStatus(isOpen bool, user *models.User) error {
	return s.db.UpdateOpenStatus(isOpen, user)
}

func (s *StatusService) UpdateTechnicalStatus(technicalStatus bool, user *models.User) error {
	return s.db.UpdateTechnicalStatus(technicalStatus, user)
}
