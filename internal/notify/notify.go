package notify

import (
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeTaskComplete  NotificationType = "task_complete"
	NotificationTypeApprovalNeeded NotificationType = "approval_needed"
	NotificationTypeError         NotificationType = "error"
	NotificationTypeRateLimit     NotificationType = "rate_limit"
	NotificationTypeInfo          NotificationType = "info"
)

// Notification represents a notification to be sent
type Notification struct {
	Type      NotificationType
	Title     string
	Message   string
	Timestamp time.Time
	Data      map[string]interface{} // Optional metadata
}

// Notifier is the interface for notification backends
type Notifier interface {
	// Notify sends a notification
	Notify(notification Notification) error
	// Close cleans up resources
	Close() error
}

// Manager manages multiple notification backends
type Manager struct {
	notifiers []Notifier
}

// NewManager creates a new notification manager
func NewManager(notifiers ...Notifier) *Manager {
	return &Manager{
		notifiers: notifiers,
	}
}

// Notify sends a notification to all registered backends
func (m *Manager) Notify(notification Notification) error {
	// Set timestamp if not provided
	if notification.Timestamp.IsZero() {
		notification.Timestamp = time.Now()
	}

	var lastErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(notification); err != nil {
			lastErr = err
			// Continue to other notifiers even if one fails
		}
	}
	return lastErr
}

// Close closes all notifiers
func (m *Manager) Close() error {
	var lastErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
