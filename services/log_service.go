package services

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	logDir         = "log"
	entriesPerPage = 5
)

type LogEntry struct {
	Timestamp  time.Time
	Action     string
	ActionData string
	ChangedBy  string
}

type LogService struct {
	logDir string
}

func NewLogService() *LogService {
	ls := &LogService{
		logDir: logDir,
	}
	ls.ensureLogDir()
	return ls
}

func (ls *LogService) ensureLogDir() {
	if _, err := os.Stat(ls.logDir); os.IsNotExist(err) {
		os.MkdirAll(ls.logDir, 0755)
	}
}

func (ls *LogService) getLogFileName(month, year int) string {
	return fmt.Sprintf("%02d.%d.log", month, year)
}

func (ls *LogService) GetLogFilePath(month, year int) string {
	return filepath.Join(ls.logDir, ls.getLogFileName(month, year))
}

func (ls *LogService) GetCurrentMonthLogPath() string {
	now := time.Now()
	return ls.GetLogFilePath(int(now.Month()), now.Year())
}

func (ls *LogService) LogStatusChange(isOpen bool, changedBy string) error {
	status := "closed"
	if isOpen {
		status = "open"
	}
	return ls.writeLogEntry("status", status, changedBy)
}

func (ls *LogService) LogKeysChange(keysAtAdmin bool, changedBy string) error {
	location := "lobby"
	if keysAtAdmin {
		location = "admin"
	}
	return ls.writeLogEntry("keys", location, changedBy)
}

func (ls *LogService) writeLogEntry(action, actionData, changedBy string) error {
	ls.ensureLogDir()

	logPath := ls.GetCurrentMonthLogPath()
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("%s|%s|%s|%s\n", timestamp, action, actionData, changedBy)

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

func (ls *LogService) GetLogEntries(month, year int, page int) ([]LogEntry, int, error) {
	logPath := ls.GetLogFilePath(month, year)

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return nil, 0, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var allEntries []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		entry, err := parseLogEntry(line)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("error reading log file: %w", err)
	}

	totalEntries := len(allEntries)
	if totalEntries == 0 {
		return nil, 0, nil
	}

	for i, j := 0, len(allEntries)-1; i < j; i, j = i+1, j-1 {
		allEntries[i], allEntries[j] = allEntries[j], allEntries[i]
	}

	totalPages := (totalEntries + entriesPerPage - 1) / entriesPerPage

	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * entriesPerPage
	endIdx := startIdx + entriesPerPage
	if endIdx > totalEntries {
		endIdx = totalEntries
	}

	return allEntries[startIdx:endIdx], totalPages, nil
}

func (ls *LogService) GetTotalPages(month, year int) (int, error) {
	logPath := ls.GetLogFilePath(month, year)

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return 0, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading log file: %w", err)
	}

	return (lineCount + entriesPerPage - 1) / entriesPerPage, nil
}

func (ls *LogService) LogFileExists(month, year int) bool {
	logPath := ls.GetLogFilePath(month, year)
	_, err := os.Stat(logPath)
	return err == nil
}

func parseLogEntry(line string) (LogEntry, error) {
	parts := strings.Split(line, "|")

	if len(parts) == 3 {
		timestamp, err := time.Parse("2006-01-02 15:04:05", parts[0])
		if err != nil {
			return LogEntry{}, fmt.Errorf("invalid timestamp format: %w", err)
		}
		return LogEntry{
			Timestamp:  timestamp,
			Action:     "status",
			ActionData: parts[1],
			ChangedBy:  parts[2],
		}, nil
	}

	if len(parts) == 4 {
		timestamp, err := time.Parse("2006-01-02 15:04:05", parts[0])
		if err != nil {
			return LogEntry{}, fmt.Errorf("invalid timestamp format: %w", err)
		}
		return LogEntry{
			Timestamp:  timestamp,
			Action:     parts[1],
			ActionData: parts[2],
			ChangedBy:  parts[3],
		}, nil
	}

	return LogEntry{}, fmt.Errorf("invalid log entry format")
}
