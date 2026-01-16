package notify

import (
	"os"
	"testing"
	"time"
)

// TestNotificationManager tests the notification manager
func TestNotificationManager(t *testing.T) {
	// Create a temporary file for summary reports
	tmpFile, err := os.CreateTemp("", "notify-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create notification backends
	terminal, err := NewTerminalNotifier()
	if err != nil {
		t.Fatalf("failed to create terminal notifier: %v", err)
	}

	summary := NewSummaryReporter(tmpFile.Name(), 100*time.Millisecond)
	summary.Start()
	defer summary.Close()

	// Create manager
	manager := NewManager(terminal, summary)

	// Test task completion notification
	err = manager.NotifyTaskComplete("bd-test", "Test Task", "Vinnie")
	if err != nil {
		t.Errorf("NotifyTaskComplete failed: %v", err)
	}

	// Test approval needed notification
	err = manager.NotifyApprovalNeeded("bd-test2", "Another Task")
	if err != nil {
		t.Errorf("NotifyApprovalNeeded failed: %v", err)
	}

	// Test error notification
	err = manager.NotifyAgentError("TestAgent", "agent-123", "Test error message")
	if err != nil {
		t.Errorf("NotifyAgentError failed: %v", err)
	}

	// Wait for summary to be generated
	time.Sleep(150 * time.Millisecond)

	// Verify summary file was created
	_, err = os.Stat(tmpFile.Name())
	if err != nil {
		t.Errorf("Summary file not created: %v", err)
	}
}

// TestTerminalNotifier tests the terminal notifier independently
func TestTerminalNotifier(t *testing.T) {
	notifier, err := NewTerminalNotifier()
	if err != nil {
		t.Fatalf("failed to create terminal notifier: %v", err)
	}

	notification := Notification{
		Type:    NotificationTypeInfo,
		Title:   "Test Notification",
		Message: "This is a test message",
	}

	err = notifier.Notify(notification)
	if err != nil {
		// On non-macOS systems, this should not error (just be a no-op)
		t.Logf("Terminal notification: %v", err)
	}
}

// TestSummaryReporter tests the summary reporter independently
func TestSummaryReporter(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "summary-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	reporter := NewSummaryReporter(tmpFile.Name(), 50*time.Millisecond)
	reporter.Start()
	defer reporter.Close()

	// Add some notifications
	for i := 0; i < 5; i++ {
		reporter.Notify(Notification{
			Type:    NotificationTypeInfo,
			Title:   "Test",
			Message: "Test message",
		})
	}

	// Wait for summary to be generated
	time.Sleep(100 * time.Millisecond)

	// Verify file exists and has content
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatalf("Summary file not found: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Summary file is empty")
	}
}
