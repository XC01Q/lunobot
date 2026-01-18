package services

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"lunobot/database"
)

type BroadcastService struct {
	db  *database.DB
	bot *tgbotapi.BotAPI
}

func NewBroadcastService(db *database.DB, bot *tgbotapi.BotAPI) *BroadcastService {
	return &BroadcastService{db: db, bot: bot}
}

func (s *BroadcastService) SendBroadcast(message string) (int, error) {
	users, err := s.db.GetAllAdmins()
	if err != nil {
		return 0, err
	}

	sentCount := 0
	for _, user := range users {
		msg := tgbotapi.NewMessage(user.TelegramID, "🚨 ALERT: "+message)
		if _, err := s.bot.Send(msg); err != nil {
			log.Printf("Failed to send broadcast to user %d: %v", user.TelegramID, err)
		} else {
			sentCount++
		}
	}

	return sentCount, nil
}

func (s *BroadcastService) SendOpenNotification() (int, error) {
	users, err := s.db.GetUsersWithNotifications()
	if err != nil {
		return 0, err
	}

	sentCount := 0
	message := "🎉 Лунотека відкрита! 🌙\n\nМожете приходити до нас!"

	for _, user := range users {
		msg := tgbotapi.NewMessage(user.TelegramID, message)
		if _, err := s.bot.Send(msg); err != nil {
			log.Printf("Failed to send open notification to user %d: %v", user.TelegramID, err)
		} else {
			sentCount++
		}
	}

	return sentCount, nil
}
