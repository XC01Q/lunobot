package services

import (
	"log"
	"lunobot/database"
	"lunobot/models"
	"strings"
	"time"
)

type SchedulerService struct {
	db               *database.DB
	statusService    *StatusService
	broadcastService *BroadcastService
	stopChan         chan struct{}
}

func NewSchedulerService(db *database.DB, statusService *StatusService, broadcastService *BroadcastService) *SchedulerService {
	return &SchedulerService{
		db:               db,
		statusService:    statusService,
		broadcastService: broadcastService,
		stopChan:         make(chan struct{}),
	}
}

func (s *SchedulerService) Start() {
	go s.run()
}

func (s *SchedulerService) Stop() {
	close(s.stopChan)
}

func (s *SchedulerService) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndExecute()
		case <-s.stopChan:
			log.Println("Scheduler stopped")
			return
		}
	}
}

func (s *SchedulerService) checkAndExecute() {
	settings, err := s.db.GetAutoCloseSettings()
	if err != nil {
		log.Printf("Error getting auto-close settings: %v", err)
		return
	}

	if !settings.Enabled {
		return
	}

	now := time.Now()
	currentTime := now.Format("15:04")

	// Check if current time matches close time (with 1-minute window)
	if s.timeMatches(currentTime, settings.CloseTime) {
		log.Printf("Auto-close triggered at %s", currentTime)
		s.executeAutoClose(settings)
	}
}

func (s *SchedulerService) timeMatches(current, target string) bool {
	// Parse times
	currentParts := strings.Split(current, ":")
	targetParts := strings.Split(target, ":")

	if len(currentParts) != 2 || len(targetParts) != 2 {
		return false
	}

	return currentParts[0] == targetParts[0] && currentParts[1] == targetParts[1]
}

func (s *SchedulerService) executeAutoClose(settings *models.AutoCloseSettings) {
	// Get current status to check if already closed
	status, err := s.statusService.GetStatus()
	if err != nil {
		log.Printf("Error getting status for auto-close: %v", err)
		return
	}

	// Skip if already closed
	if !status.IsOpen {
		return
	}

	// Use the last user who changed status
	updatedBy := settings.LastStatusBy
	if updatedBy == "" {
		updatedBy = "auto-close"
	}

	// Close the status
	err = s.db.UpdateOpenStatusAuto(false, settings.KeysToLobby, updatedBy)
	if err != nil {
		log.Printf("Error executing auto-close: %v", err)
		return
	}

	log.Printf("Auto-close executed. Status set to closed by %s, keys to lobby: %v", updatedBy, settings.KeysToLobby)
}

func (s *SchedulerService) GetSettings() (*models.AutoCloseSettings, error) {
	return s.db.GetAutoCloseSettings()
}

func (s *SchedulerService) UpdateSettings(enabled bool, closeTime string, keysToLobby bool) error {
	return s.db.UpdateAutoCloseSettings(enabled, closeTime, keysToLobby)
}

func (s *SchedulerService) UpdateLastUser(username string) error {
	return s.db.UpdateAutoCloseLastUser(username)
}
