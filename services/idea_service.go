package services

import (
	"lunobot/database"
	"lunobot/models"
)

type IdeaService struct {
	db *database.DB
}

func NewIdeaService(db *database.DB) *IdeaService {
	return &IdeaService{db: db}
}

func (s *IdeaService) AddIdea(userID int64, username, content string) error {
	idea := &models.Idea{
		UserID:   userID,
		Username: username,
		Content:  content,
	}
	return s.db.AddIdea(idea)
}

func (s *IdeaService) GetAllIdeas() ([]models.Idea, error) {
	return s.db.GetAllIdeas()
}

func (s *IdeaService) DeleteIdea(ideaID int64) error {
	return s.db.DeleteIdea(ideaID)
}
