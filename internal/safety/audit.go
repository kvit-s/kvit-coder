package safety

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuditEntry represents a log entry for blocked commands
type AuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"session_id"`
	Command    string    `json:"command"`
	Action     string    `json:"action"`
	Rule       string    `json:"rule"`
	Message    string    `json:"message"`
	WorkingDir string    `json:"working_dir,omitempty"`
}

// AuditLogger logs blocked commands to JSONL files
type AuditLogger struct {
	config    AuditConfig
	sessionID string
	logFile   *os.File
	redactor  *Redactor
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(cfg AuditConfig) *AuditLogger {
	if !cfg.Enabled {
		return nil
	}

	// Expand ~ in log_dir
	logDir := expandPath(cfg.LogDir)
	if logDir == "" {
		logDir = filepath.Join(os.Getenv("HOME"), ".kvit-coder", "safety-logs")
	}

	// Create log directory
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil
	}

	sessionID := generateSessionID()
	logPath := filepath.Join(logDir, sessionID+".jsonl")

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil
	}

	var redactor *Redactor
	if cfg.RedactSecrets {
		redactor = NewRedactor()
	}

	return &AuditLogger{
		config:    cfg,
		sessionID: sessionID,
		logFile:   file,
		redactor:  redactor,
	}
}

// LogBlocked logs a blocked command
func (a *AuditLogger) LogBlocked(cmd string, result *RuleResult, workingDir string) error {
	if a == nil || a.logFile == nil {
		return nil
	}

	// Redact sensitive data if configured
	redactedCmd := cmd
	if a.redactor != nil {
		redactedCmd = a.redactor.Redact(cmd)
	}

	entry := AuditEntry{
		Timestamp:  time.Now().UTC(),
		SessionID:  a.sessionID,
		Command:    redactedCmd,
		Action:     "blocked",
		Rule:       result.Rule,
		Message:    result.Message,
		WorkingDir: workingDir,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = a.logFile.WriteString(string(line) + "\n")
	return err
}

// Close closes the audit log file
func (a *AuditLogger) Close() error {
	if a != nil && a.logFile != nil {
		return a.logFile.Close()
	}
	return nil
}

// SessionID returns the current session ID
func (a *AuditLogger) SessionID() string {
	if a == nil {
		return ""
	}
	return a.sessionID
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	// Format: YYYY-MM-DD-HHMMSS-RANDOM
	now := time.Now()
	dateStr := now.Format("2006-01-02-150405")

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp-based
		return dateStr
	}

	return dateStr + "-" + hex.EncodeToString(randomBytes)
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	return path
}
