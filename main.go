package main

import (
	"context"
	"log"
	"lunobot/config"
	"lunobot/database"
	"lunobot/handlers"
	"lunobot/services"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Load()
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// Initialize database
	db, err := database.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}
	bot.Debug = cfg.Debug

	log.Printf("Bot %s started successfully!", bot.Self.UserName)

	// Initialize services
	userService := services.NewUserService(db)
	ideaService := services.NewIdeaService(db)
	statusService := services.NewStatusService(db)
	broadcastService := services.NewBroadcastService(db, bot)
	schedulerService := services.NewSchedulerService(db, statusService, broadcastService)

	// Initialize handlers
	botHandlers := handlers.NewBotHandlers(bot, userService, ideaService, statusService, broadcastService, schedulerService)

	// Start scheduler
	schedulerService.Start()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("Shutting down bot...")
		schedulerService.Stop()
		cancel()
	}()

	// Start bot
	botHandlers.Start(ctx)
}
