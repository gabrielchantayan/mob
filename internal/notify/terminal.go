package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// TerminalNotifier sends notifications via macOS terminal notifications
type TerminalNotifier struct {
	enabled bool
}

// NewTerminalNotifier creates a new terminal notifier
func NewTerminalNotifier() (*TerminalNotifier, error) {
	// Only enable on macOS
	if runtime.GOOS != "darwin" {
		return &TerminalNotifier{enabled: false}, nil
	}

	return &TerminalNotifier{enabled: true}, nil
}

// Notify sends a terminal notification using osascript
func (t *TerminalNotifier) Notify(notification Notification) error {
	if !t.enabled {
		return nil
	}

	// Escape quotes in title and message
	title := escapeAppleScript(notification.Title)
	message := escapeAppleScript(notification.Message)

	// Build the AppleScript command
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)

	// Execute osascript
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send terminal notification: %w", err)
	}

	return nil
}

// Close cleans up resources (no-op for terminal notifier)
func (t *TerminalNotifier) Close() error {
	return nil
}

// escapeAppleScript escapes quotes and backslashes for AppleScript
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
