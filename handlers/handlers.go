package handlers

import (
	"context"
	"fmt"
	"log"
	"lunobot/i18n"
	"lunobot/menu"
	"lunobot/models"
	"lunobot/services"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UserState struct {
	State   string
	Data    map[string]interface{}
	Expires time.Time
}

type BotHandlers struct {
	bot              *tgbotapi.BotAPI
	userService      *services.UserService
	ideaService      *services.IdeaService
	statusService    *services.StatusService
	broadcastService *services.BroadcastService
	schedulerService *services.SchedulerService
	menu             *menu.MenuGenerator
	translator       *i18n.Translator
	userStates       map[int64]*UserState
	stateMutex       sync.RWMutex
}

func NewBotHandlers(
	bot *tgbotapi.BotAPI,
	userService *services.UserService,
	ideaService *services.IdeaService,
	statusService *services.StatusService,
	broadcastService *services.BroadcastService,
	schedulerService *services.SchedulerService,
) *BotHandlers {
	translator := i18n.NewTranslator()
	h := &BotHandlers{
		bot:              bot,
		userService:      userService,
		ideaService:      ideaService,
		statusService:    statusService,
		broadcastService: broadcastService,
		schedulerService: schedulerService,
		menu:             menu.NewMenuGenerator(translator),
		translator:       translator,
		userStates:       make(map[int64]*UserState),
	}
	go h.cleanupExpiredStates()
	return h
}

func (h *BotHandlers) getUserLang(user *models.User) i18n.Language {
	return i18n.ParseLanguage(user.Language)
}

func (h *BotHandlers) t(key string, user *models.User) string {
	return h.translator.Get(key, h.getUserLang(user))
}

func (h *BotHandlers) tParams(key string, user *models.User, params map[string]string) string {
	return h.translator.GetWithParams(key, h.getUserLang(user), params)
}

func (h *BotHandlers) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)
	for {
		select {
		case update := <-updates:
			go h.HandleUpdate(update)
		case <-ctx.Done():
			log.Println("Bot stopped")
			return
		}
	}
}

func (h *BotHandlers) HandleUpdate(update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in HandleUpdate: %v", r)
		}
	}()
	if update.Message != nil {
		h.handleMessage(update.Message)
	} else if update.CallbackQuery != nil {
		h.handleCallback(update.CallbackQuery)
	}
}

func (h *BotHandlers) handleMessage(message *tgbotapi.Message) {
	user, err := h.userService.GetOrCreateUser(
		message.From.ID,
		message.From.UserName,
		message.From.FirstName,
		message.From.LastName,
	)
	if err != nil {
		log.Printf("Error getting/creating user: %v", err)
		h.sendMessage(message.Chat.ID, "❌ An error occurred. Please try again later.")
		return
	}
	switch message.Command() {
	case "start":
		h.clearUserState(message.From.ID)
		h.sendLanguageSelection(message.Chat.ID)
	case "help":
		h.sendHelpMessage(message.Chat.ID, user)
	case "cancel":
		h.clearUserState(message.From.ID)
		h.sendMainMenu(message.Chat.ID, user)
	default:
		if state := h.getUserState(message.From.ID); state != nil {
			h.handleUserState(message, state, user)
		} else {
			h.sendMainMenu(message.Chat.ID, user)
		}
	}
}

func (h *BotHandlers) handleCallback(callback *tgbotapi.CallbackQuery) {
	h.answerCallback(callback.ID, "")

	user, err := h.userService.GetOrCreateUser(
		callback.From.ID,
		callback.From.UserName,
		callback.From.FirstName,
		callback.From.LastName,
	)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		return
	}

	h.routeCallback(callback, user)
}

func (h *BotHandlers) routeCallback(callback *tgbotapi.CallbackQuery, user *models.User) {
	data := callback.Data
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	switch {
	case strings.HasPrefix(data, "lang_"):
		h.handleLanguageSelection(data, chatID, messageID, user)
	case data == "change_language":
		h.handleChangeLanguage(chatID, messageID)
	case data == "check_status":
		h.handleCheckStatus(chatID, messageID, user)
	case data == "send_idea":
		h.handleSendIdea(callback.From.ID, chatID, messageID, user)
	case data == "notifications":
		h.handleNotifications(chatID, messageID, user)
	case data == "notifications_toggle":
		h.handleNotificationsToggle(chatID, messageID, user)
	case data == "set_open_status" && user.HasRights(models.RightsManager):
		h.handleSetOpenStatus(chatID, messageID, user)
	case data == "set_tech_status" && user.HasRights(models.RightsManager):
		h.handleSetTechStatus(chatID, messageID, user)
	case data == "read_ideas" && user.HasRights(models.RightsAdmin):
		h.handleReadIdeas(chatID, messageID, user)
	case data == "set_rights" && user.HasRights(models.RightsAdmin):
		h.handleSetRights(chatID, messageID, user)
	case data == "create_broadcast" && user.HasRights(models.RightsAdmin):
		h.handleCreateBroadcast(callback.From.ID, chatID, messageID, user)
	case data == "auto_close" && user.HasRights(models.RightsAdmin):
		h.handleAutoCloseSettings(chatID, messageID, user)
	case data == "auto_close_toggle" && user.HasRights(models.RightsAdmin):
		h.handleAutoCloseToggle(chatID, messageID, user)
	case data == "auto_close_time" && user.HasRights(models.RightsAdmin):
		h.handleAutoCloseTimePrompt(callback.From.ID, chatID, messageID, user)
	case data == "auto_close_keys" && user.HasRights(models.RightsAdmin):
		h.handleAutoCloseKeysSelect(chatID, messageID, user)
	case strings.HasPrefix(data, "autoclose_keys_") && user.HasRights(models.RightsAdmin):
		h.handleAutoCloseKeysUpdate(data, chatID, messageID, user)
	case data == "back_to_menu":
		h.sendMainMenu(chatID, user)
		h.deleteMessage(chatID, messageID)
	case strings.HasPrefix(data, "open_") && user.HasRights(models.RightsManager):
		h.handleOpenStatusUpdate(data, chatID, messageID, user)
	case strings.HasPrefix(data, "tech_") && user.HasRights(models.RightsManager):
		h.handleTechStatusUpdate(data, chatID, messageID, user)
	case strings.HasPrefix(data, "rights_") && user.HasRights(models.RightsAdmin):
		h.handleRightsSelection(data, callback.From.ID, chatID, messageID, user)
	case strings.HasPrefix(data, "idea_") && user.HasRights(models.RightsAdmin):
		h.handleIdeaAction(data, callback, user)
	default:
		h.sendMessage(chatID, h.t("error_unknown_command", user))
	}
}

func (h *BotHandlers) handleUserState(message *tgbotapi.Message, state *UserState, user *models.User) {
	chatID := message.Chat.ID
	userID := message.From.ID
	switch state.State {
	case "waiting_idea":
		if len(message.Text) > 4000 {
			h.sendMessage(chatID, h.t("idea_too_long", user))
			return
		}
		if err := h.ideaService.AddIdea(userID, message.From.UserName, message.Text); err != nil {
			h.sendMessage(chatID, h.tParams("error_save_idea", user, map[string]string{"error": err.Error()}))
		} else {
			h.sendMessage(chatID, h.t("idea_saved", user))
		}
		h.clearUserState(userID)
		h.sendMainMenu(chatID, user)
	case "waiting_username":
		rightsStr, ok := state.Data["rights"].(string)
		if !ok {
			h.sendMessage(chatID, h.t("error_data_processing", user))
			h.clearUserState(userID)
			return
		}
		rightsInt, err := strconv.Atoi(rightsStr)
		if err != nil {
			h.sendMessage(chatID, h.t("error_invalid_rights", user))
			h.clearUserState(userID)
			return
		}
		rights := models.Rights(rightsInt)
		if !rights.IsValid() {
			h.sendMessage(chatID, h.t("error_invalid_rights", user))
			h.clearUserState(userID)
			return
		}

		username := strings.TrimSpace(message.Text)
		if username == "" {
			h.sendMessage(chatID, h.t("error_empty_username", user))
			return
		}

		targetUser, err := h.userService.GetUserByUsername(username)
		if err != nil {
			if err == models.ErrUserNotFound {
				h.sendMessage(chatID, h.t("user_not_found", user))
			} else {
				h.sendMessage(chatID, h.tParams("error_find_user", user, map[string]string{"error": err.Error()}))
			}
			h.clearUserState(userID)
			return
		}

		if err := h.userService.UpdateUserRightsByUsername(username, rights); err != nil {
			h.sendMessage(chatID, h.tParams("error_update_rights", user, map[string]string{"error": err.Error()}))
		} else {
			h.sendMessage(chatID, h.tParams("rights_updated", user, map[string]string{
				"user":   targetUser.GetDisplayName(),
				"rights": h.getRightsName(rights, user),
			}))
		}
		h.clearUserState(userID)
		h.sendMainMenu(chatID, user)
	case "waiting_broadcast":
		if len(message.Text) > 4000 {
			h.sendMessage(chatID, h.t("idea_too_long", user))
			return
		}
		sentCount, err := h.broadcastService.SendBroadcast(message.Text)
		if err != nil {
			h.sendMessage(chatID, h.tParams("error_broadcast", user, map[string]string{"error": err.Error()}))
		} else {
			h.sendMessage(chatID, h.tParams("broadcast_sent", user, map[string]string{"count": strconv.Itoa(sentCount)}))
		}
		h.clearUserState(userID)
		h.sendMainMenu(chatID, user)
	case "waiting_auto_close_time":
		timeStr := strings.TrimSpace(message.Text)
		h.clearUserState(userID)
		h.handleAutoCloseTimeUpdate(chatID, user, timeStr)
	}
}

func (h *BotHandlers) getRightsName(rights models.Rights, user *models.User) string {
	lang := h.getUserLang(user)
	switch rights {
	case models.RightsDefault:
		return h.translator.Get("rights_user", lang)
	case models.RightsManager:
		return h.translator.Get("rights_manager", lang)
	case models.RightsAdmin:
		return h.translator.Get("rights_admin", lang)
	default:
		return h.translator.Get("rights_unknown", lang)
	}
}

func (h *BotHandlers) setUserState(userID int64, state string, data map[string]interface{}) {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()

	h.userStates[userID] = &UserState{
		State:   state,
		Data:    data,
		Expires: time.Now().Add(10 * time.Minute),
	}
}

func (h *BotHandlers) getUserState(userID int64) *UserState {
	h.stateMutex.RLock()
	defer h.stateMutex.RUnlock()

	state, exists := h.userStates[userID]
	if !exists || time.Now().After(state.Expires) {
		return nil
	}
	return state
}

func (h *BotHandlers) clearUserState(userID int64) {
	h.stateMutex.Lock()
	defer h.stateMutex.Unlock()
	delete(h.userStates, userID)
}

func (h *BotHandlers) cleanupExpiredStates() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		h.stateMutex.Lock()
		now := time.Now()
		for userID, state := range h.userStates {
			if now.After(state.Expires) {
				delete(h.userStates, userID)
			}
		}
		h.stateMutex.Unlock()
	}
}

func (h *BotHandlers) sendLanguageSelection(chatID int64) {
	keyboard := h.menu.GenerateLanguageKeyboard()
	msg := tgbotapi.NewMessage(chatID, "🌍 Оберіть мову / Select language:")
	msg.ReplyMarkup = keyboard
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending language selection: %v", err)
	}
}

func (h *BotHandlers) handleLanguageSelection(data string, chatID int64, messageID int, user *models.User) {
	langCode := strings.TrimPrefix(data, "lang_")
	lang := i18n.ParseLanguage(langCode)

	if err := h.userService.UpdateUserLanguage(user.TelegramID, lang.String()); err != nil {
		log.Printf("Error updating user language: %v", err)
	}

	user.Language = lang.String()

	h.deleteMessage(chatID, messageID)
	h.sendWelcomeMessage(chatID, user)
}

func (h *BotHandlers) handleChangeLanguage(chatID int64, messageID int) {
	keyboard := h.menu.GenerateLanguageKeyboard()
	h.editMessageWithKeyboard(chatID, messageID, "🌍 Оберіть мову / Select language:", keyboard)
}

func (h *BotHandlers) sendWelcomeMessage(chatID int64, user *models.User) {
	welcomeText := h.tParams("welcome", user, map[string]string{
		"display_name": user.GetDisplayName(),
		"rights":       h.getRightsName(user.Rights, user),
	})
	h.sendMessage(chatID, welcomeText)
	h.sendMainMenu(chatID, user)
}

func (h *BotHandlers) sendHelpMessage(chatID int64, user *models.User) {
	helpText := h.t("help_header", user)
	helpText += h.t("help_commands", user)

	if user.HasRights(models.RightsDefault) {
		helpText += h.t("help_user_features", user)
	}

	if user.HasRights(models.RightsManager) {
		helpText += h.t("help_manager_features", user)
	}

	if user.HasRights(models.RightsAdmin) {
		helpText += h.t("help_admin_features", user)
	}
	h.sendMessage(chatID, helpText)
}

func (h *BotHandlers) sendMainMenu(chatID int64, user *models.User) {
	lang := h.getUserLang(user)
	keyboard := h.menu.GenerateKeyboard(user.Rights, lang)
	msg := tgbotapi.NewMessage(chatID, h.t("main_menu", user))
	msg.ReplyMarkup = keyboard
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending main menu: %v", err)
	}
}

func (h *BotHandlers) handleCheckStatus(chatID int64, messageID int, user *models.User) {
	status, err := h.statusService.GetStatus()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_get_status", user))
		return
	}
	statusText := h.formatStatus(status, user)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_refresh", user), "check_status"),
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_back", user), "back_to_menu"),
		),
	)
	h.editMessageWithKeyboard(chatID, messageID, statusText, keyboard)
}

func (h *BotHandlers) formatStatus(status *models.Status, user *models.User) string {
	openStatus := h.t("status_closed", user)
	if status.IsOpen {
		openStatus = h.t("status_open", user)
	}
	keyLocation := h.t("status_keys_lobby", user)
	if status.TechnicalStatus {
		keyLocation = h.t("status_keys_admin", user)
	}

	text := h.t("status_header", user)
	text += h.tParams("status_state", user, map[string]string{"status": openStatus}) + "\n"
	text += h.tParams("status_keys", user, map[string]string{"location": keyLocation}) + "\n"
	text += h.tParams("status_updated", user, map[string]string{"time": status.UpdatedAt.Format("02.01.2006 15:04")})

	if user.Rights >= models.RightsManager {
		text += "\n" + h.tParams("status_updated_by", user, map[string]string{"user": status.UpdatedBy})
	}
	return text
}

func (h *BotHandlers) handleNotifications(chatID int64, messageID int, user *models.User) {
	var statusText string
	if user.NotificationsEnabled {
		statusText = h.t("notifications_enabled", user)
	} else {
		statusText = h.t("notifications_disabled", user)
	}

	lang := h.getUserLang(user)
	keyboard := h.menu.GenerateNotificationsKeyboard(user.NotificationsEnabled, lang)
	h.editMessageWithKeyboard(chatID, messageID, statusText, keyboard)
}

func (h *BotHandlers) handleNotificationsToggle(chatID int64, messageID int, user *models.User) {
	newStatus := !user.NotificationsEnabled

	if err := h.userService.UpdateNotificationsEnabled(user.TelegramID, newStatus); err != nil {
		h.editMessage(chatID, messageID, h.t("error_notifications", user))
		return
	}

	user.NotificationsEnabled = newStatus
	h.handleNotifications(chatID, messageID, user)
}

func (h *BotHandlers) handleSendIdea(userID, chatID int64, messageID int, user *models.User) {
	h.setUserState(userID, "waiting_idea", nil)
	h.editMessage(chatID, messageID, h.t("idea_prompt", user))
}

func (h *BotHandlers) handleSetOpenStatus(chatID int64, messageID int, user *models.User) {
	lang := h.getUserLang(user)
	keyboard := h.menu.GenerateStatusKeyboard("open", lang)
	h.editMessageWithKeyboard(chatID, messageID, h.t("status_select_open", user), keyboard)
}

func (h *BotHandlers) handleSetTechStatus(chatID int64, messageID int, user *models.User) {
	lang := h.getUserLang(user)
	keyboard := h.menu.GenerateStatusKeyboard("tech", lang)
	h.editMessageWithKeyboard(chatID, messageID, h.t("status_select_keys", user), keyboard)
}

func (h *BotHandlers) handleOpenStatusUpdate(data string, chatID int64, messageID int, user *models.User) {
	isOpen := strings.TrimPrefix(data, "open_") == "true"

	if err := h.statusService.UpdateOpenStatus(isOpen, user); err != nil {
		h.editMessage(chatID, messageID, h.t("error_update_status", user))
		return
	}

	// Track last user who changed status for auto-close
	h.schedulerService.UpdateLastUser(user.GetDisplayName())

	statusText := h.t("status_changed_closed", user)
	if isOpen {
		statusText = h.t("status_changed_open", user)
		go func() {
			if count, err := h.broadcastService.SendOpenNotification(); err != nil {
				log.Printf("Error sending open notifications: %v", err)
			} else {
				log.Printf("Open notifications sent to %d users", count)
			}
		}()
	}

	h.editMessage(chatID, messageID, statusText)

	go func() {
		time.Sleep(2 * time.Second)
		h.sendMainMenu(chatID, user)
		h.deleteMessage(chatID, messageID)
	}()
}

func (h *BotHandlers) handleTechStatusUpdate(data string, chatID int64, messageID int, user *models.User) {
	techStatus := strings.TrimPrefix(data, "tech_") == "true"

	if err := h.statusService.UpdateTechnicalStatus(techStatus, user); err != nil {
		h.editMessage(chatID, messageID, h.t("error_update_keys", user))
		return
	}

	location := h.t("keys_location_lobby", user)
	if techStatus {
		location = h.t("keys_location_admin", user)
	}
	h.editMessage(chatID, messageID, h.tParams("status_keys_changed", user, map[string]string{"location": location}))

	go func() {
		time.Sleep(2 * time.Second)
		h.sendMainMenu(chatID, user)
		h.deleteMessage(chatID, messageID)
	}()
}

func (h *BotHandlers) handleReadIdeas(chatID int64, messageID int, user *models.User) {
	ideas, err := h.ideaService.GetAllIdeas()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_get_ideas", user))
		return
	}
	if len(ideas) == 0 {
		h.editMessage(chatID, messageID, h.t("ideas_empty", user))
		time.Sleep(2 * time.Second)
		h.sendMainMenu(chatID, user)
		h.deleteMessage(chatID, messageID)
		return
	}
	h.showIdea(chatID, messageID, ideas, 0, user)
}

func (h *BotHandlers) showIdea(chatID int64, messageID int, ideas []models.Idea, currentIndex int, user *models.User) {
	if currentIndex < 0 || currentIndex >= len(ideas) {
		return
	}
	idea := ideas[currentIndex]
	username := idea.Username
	if username == "" {
		username = h.t("idea_anonymous", user)
	}
	text := h.tParams("idea_header", user, map[string]string{
		"current":  strconv.Itoa(currentIndex + 1),
		"total":    strconv.Itoa(len(ideas)),
		"id":       strconv.FormatInt(idea.ID, 10),
		"username": username,
		"date":     idea.CreatedAt.Format("02.01.2006 15:04"),
		"content":  idea.Content,
	})
	keyboard := h.generateIdeaKeyboard(currentIndex, len(ideas), idea.ID, user)
	h.editMessageWithKeyboard(chatID, messageID, text, keyboard)
}

func (h *BotHandlers) generateIdeaKeyboard(currentIndex, totalIdeas int, ideaID int64, user *models.User) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var navRow []tgbotapi.InlineKeyboardButton
	if currentIndex > 0 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️", fmt.Sprintf("idea_prev_%d", currentIndex)))
	}
	if currentIndex < totalIdeas-1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("➡️", fmt.Sprintf("idea_next_%d", currentIndex)))
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}
	actionRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(h.t("btn_delete_idea", user), fmt.Sprintf("idea_delete_%d", ideaID)),
	}
	rows = append(rows, actionRow)
	backRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(h.t("btn_back", user), "back_to_menu"),
	}
	rows = append(rows, backRow)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (h *BotHandlers) handleIdeaAction(data string, callback *tgbotapi.CallbackQuery, user *models.User) {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	if strings.HasPrefix(data, "idea_prev_") || strings.HasPrefix(data, "idea_next_") {
		h.handleIdeaNavigation(data, chatID, messageID, user)
	} else if strings.HasPrefix(data, "idea_delete_") {
		h.handleIdeaDelete(data, chatID, messageID, user)
	}
}

func (h *BotHandlers) handleIdeaNavigation(data string, chatID int64, messageID int, user *models.User) {
	ideas, err := h.ideaService.GetAllIdeas()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_get_ideas", user))
		return
	}
	var newIndex int
	if strings.HasPrefix(data, "idea_prev_") {
		currentIndex, _ := strconv.Atoi(strings.TrimPrefix(data, "idea_prev_"))
		newIndex = currentIndex - 1
	} else if strings.HasPrefix(data, "idea_next_") {
		currentIndex, _ := strconv.Atoi(strings.TrimPrefix(data, "idea_next_"))
		newIndex = currentIndex + 1
	}
	h.showIdea(chatID, messageID, ideas, newIndex, user)
}

func (h *BotHandlers) handleIdeaDelete(data string, chatID int64, messageID int, user *models.User) {
	ideaIDStr := strings.TrimPrefix(data, "idea_delete_")
	ideaID, err := strconv.ParseInt(ideaIDStr, 10, 64)
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_idea_id", user))
		return
	}
	if err := h.ideaService.DeleteIdea(ideaID); err != nil {
		if err == models.ErrIdeaNotFound {
			h.editMessage(chatID, messageID, h.t("idea_not_found", user))
		} else {
			h.editMessage(chatID, messageID, h.t("error_delete_idea", user))
		}
		return
	}
	h.editMessage(chatID, messageID, h.t("idea_deleted", user))

	go func() {
		time.Sleep(2 * time.Second)
		h.sendMainMenu(chatID, user)
		h.deleteMessage(chatID, messageID)
	}()
}

func (h *BotHandlers) handleSetRights(chatID int64, messageID int, user *models.User) {
	lang := h.getUserLang(user)
	keyboard := h.menu.GenerateRightsKeyboard(lang)
	h.editMessageWithKeyboard(chatID, messageID, h.t("rights_select", user), keyboard)
}

func (h *BotHandlers) handleRightsSelection(data string, userID, chatID int64, messageID int, user *models.User) {
	rightsStr := strings.TrimPrefix(data, "rights_")
	h.setUserState(userID, "waiting_username", map[string]interface{}{
		"rights": rightsStr,
	})
	h.editMessage(chatID, messageID, h.t("rights_username_prompt", user))
}

func (h *BotHandlers) handleCreateBroadcast(userID, chatID int64, messageID int, user *models.User) {
	h.setUserState(userID, "waiting_broadcast", nil)
	h.editMessage(chatID, messageID, h.t("broadcast_prompt", user))
}

func (h *BotHandlers) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (h *BotHandlers) editMessage(chatID int64, messageID int, text string) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if _, err := h.bot.Send(edit); err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("Error editing message: %v", err)
		}
	}
}

func (h *BotHandlers) editMessageWithKeyboard(chatID int64, messageID int, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, text, keyboard)
	if _, err := h.bot.Send(edit); err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("Error editing message with keyboard: %v", err)
		}
	}
}

func (h *BotHandlers) deleteMessage(chatID int64, messageID int) {
	deleteConfig := tgbotapi.DeleteMessageConfig{
		ChatID:    chatID,
		MessageID: messageID,
	}

	h.bot.Request(deleteConfig)
}

func (h *BotHandlers) answerCallback(callbackID, text string) {
	callback := tgbotapi.CallbackConfig{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       false,
	}

	h.bot.Request(callback)
}

// Auto-close handlers

func (h *BotHandlers) handleAutoCloseSettings(chatID int64, messageID int, user *models.User) {
	settings, err := h.schedulerService.GetSettings()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_generic", user))
		return
	}

	status := h.t("auto_close_status_disabled", user)
	if settings.Enabled {
		status = h.t("auto_close_status_enabled", user)
	}

	keysLocation := h.t("status_keys_lobby", user)
	if !settings.KeysToLobby {
		keysLocation = h.t("status_keys_admin", user)
	}

	text := h.tParams("auto_close_info", user, map[string]string{
		"status":    status,
		"time":      settings.CloseTime,
		"keys":      keysLocation,
		"last_user": settings.LastStatusBy,
	})

	toggleAction := h.t("auto_close_toggle_enable", user)
	if settings.Enabled {
		toggleAction = h.t("auto_close_toggle_disable", user)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.tParams("btn_auto_close_toggle", user, map[string]string{"action": toggleAction}), "auto_close_toggle"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_auto_close_time", user), "auto_close_time"),
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_auto_close_keys", user), "auto_close_keys"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_back", user), "back_to_menu"),
		),
	)

	h.editMessageWithKeyboard(chatID, messageID, text, keyboard)
}

func (h *BotHandlers) handleAutoCloseToggle(chatID int64, messageID int, user *models.User) {
	settings, err := h.schedulerService.GetSettings()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_generic", user))
		return
	}

	newEnabled := !settings.Enabled
	if err := h.schedulerService.UpdateSettings(newEnabled, settings.CloseTime, settings.KeysToLobby); err != nil {
		h.editMessage(chatID, messageID, h.t("error_generic", user))
		return
	}

	// Show updated settings
	h.handleAutoCloseSettings(chatID, messageID, user)
}

func (h *BotHandlers) handleAutoCloseTimePrompt(userID, chatID int64, messageID int, user *models.User) {
	h.setUserState(userID, "waiting_auto_close_time", nil)
	h.editMessage(chatID, messageID, h.t("auto_close_time_prompt", user))
}

func (h *BotHandlers) handleAutoCloseKeysSelect(chatID int64, messageID int, user *models.User) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.t("status_btn_keys_lobby", user), "autoclose_keys_lobby"),
			tgbotapi.NewInlineKeyboardButtonData(h.t("status_btn_keys_admin", user), "autoclose_keys_admin"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.t("btn_back", user), "auto_close"),
		),
	)
	h.editMessageWithKeyboard(chatID, messageID, h.t("auto_close_keys_select", user), keyboard)
}

func (h *BotHandlers) handleAutoCloseKeysUpdate(data string, chatID int64, messageID int, user *models.User) {
	keysToLobby := strings.TrimPrefix(data, "autoclose_keys_") == "lobby"

	settings, err := h.schedulerService.GetSettings()
	if err != nil {
		h.editMessage(chatID, messageID, h.t("error_generic", user))
		return
	}

	if err := h.schedulerService.UpdateSettings(settings.Enabled, settings.CloseTime, keysToLobby); err != nil {
		h.editMessage(chatID, messageID, h.t("error_generic", user))
		return
	}

	location := h.t("keys_location_lobby", user)
	if !keysToLobby {
		location = h.t("keys_location_admin", user)
	}

	h.editMessage(chatID, messageID, h.tParams("auto_close_keys_updated", user, map[string]string{"location": location}))

	go func() {
		time.Sleep(2 * time.Second)
		h.handleAutoCloseSettings(chatID, messageID, user)
	}()
}

func (h *BotHandlers) handleAutoCloseTimeUpdate(chatID int64, user *models.User, timeStr string) {
	// Validate time format HH:MM
	if !h.isValidTimeFormat(timeStr) {
		h.sendMessage(chatID, h.t("auto_close_time_invalid", user))
		return
	}

	settings, err := h.schedulerService.GetSettings()
	if err != nil {
		h.sendMessage(chatID, h.t("error_generic", user))
		return
	}

	if err := h.schedulerService.UpdateSettings(settings.Enabled, timeStr, settings.KeysToLobby); err != nil {
		h.sendMessage(chatID, h.t("error_generic", user))
		return
	}

	h.sendMessage(chatID, h.tParams("auto_close_time_updated", user, map[string]string{"time": timeStr}))
	h.sendMainMenu(chatID, user)
}

func (h *BotHandlers) isValidTimeFormat(timeStr string) bool {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return false
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return false
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return false
	}

	return true
}
