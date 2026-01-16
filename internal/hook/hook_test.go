package hook

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHook_Marshal(t *testing.T) {
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	hook := &Hook{
		Type:      HookTypeAssign,
		BeadID:    "bd-1234",
		Message:   "New task assigned",
		Timestamp: timestamp,
		Seq:       1,
	}

	data, err := json.Marshal(hook)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify JSON structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if decoded["type"] != "assign" {
		t.Errorf("expected type 'assign', got %v", decoded["type"])
	}
	if decoded["bead_id"] != "bd-1234" {
		t.Errorf("expected bead_id 'bd-1234', got %v", decoded["bead_id"])
	}
	if decoded["message"] != "New task assigned" {
		t.Errorf("expected message 'New task assigned', got %v", decoded["message"])
	}
	if decoded["seq"] != float64(1) {
		t.Errorf("expected seq 1, got %v", decoded["seq"])
	}

	// Verify round-trip
	var roundTrip Hook
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("Unmarshal to Hook failed: %v", err)
	}
	if roundTrip.Type != HookTypeAssign {
		t.Errorf("expected type HookTypeAssign, got %v", roundTrip.Type)
	}
	if roundTrip.BeadID != "bd-1234" {
		t.Errorf("expected bead_id 'bd-1234', got %v", roundTrip.BeadID)
	}
	if roundTrip.Seq != 1 {
		t.Errorf("expected seq 1, got %v", roundTrip.Seq)
	}
}

func TestManager_Write(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	hook := &Hook{
		Type:      HookTypeAssign,
		BeadID:    "bd-1234",
		Message:   "Work assignment",
		Timestamp: time.Now(),
		Seq:       1,
	}

	if err := mgr.Write(hook); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file was created
	hookPath := filepath.Join(tmpDir, "vinnie", "hook.json")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Errorf("expected hook file to exist at %s", hookPath)
	}

	// Verify contents
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var readHook Hook
	if err := json.Unmarshal(data, &readHook); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if readHook.Type != HookTypeAssign {
		t.Errorf("expected type HookTypeAssign, got %v", readHook.Type)
	}
	if readHook.BeadID != "bd-1234" {
		t.Errorf("expected bead_id 'bd-1234', got %v", readHook.BeadID)
	}
}

func TestManager_Read(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Write a hook first
	originalHook := &Hook{
		Type:      HookTypeNudge,
		Message:   "Wake up",
		Timestamp: time.Now(),
		Seq:       0, // Will be auto-incremented to 1
	}

	if err := mgr.Write(originalHook); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read it back
	readHook, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if readHook.Type != HookTypeNudge {
		t.Errorf("expected type HookTypeNudge, got %v", readHook.Type)
	}
	if readHook.Message != "Wake up" {
		t.Errorf("expected message 'Wake up', got %v", readHook.Message)
	}
	// Seq is auto-incremented, first write gets seq=1
	if readHook.Seq != 1 {
		t.Errorf("expected seq 1 (auto-incremented), got %v", readHook.Seq)
	}
}

func TestManager_Clear(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Write a hook first
	hook := &Hook{
		Type:      HookTypeAssign,
		BeadID:    "bd-1234",
		Timestamp: time.Now(),
		Seq:       1,
	}

	if err := mgr.Write(hook); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	hookPath := mgr.Path()
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Fatalf("hook file should exist before clear")
	}

	// Clear the hook
	if err := mgr.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Errorf("expected hook file to be removed after clear")
	}
}

func TestManager_ReadNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Read without writing first - should return nil, nil (no hook)
	hook, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read should not error on missing file, got: %v", err)
	}
	if hook != nil {
		t.Errorf("expected nil hook when file doesn't exist, got %v", hook)
	}
}

func TestManager_Path(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "vinnie", "hook.json")
	actualPath := mgr.Path()

	if actualPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, actualPath)
	}
}

func TestManager_WriteIncrementsSeq(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Write first hook with seq 0 - should be incremented to 1
	hook1 := &Hook{
		Type:      HookTypeAssign,
		BeadID:    "bd-0001",
		Timestamp: time.Now(),
		Seq:       0, // Will be auto-incremented
	}

	if err := mgr.Write(hook1); err != nil {
		t.Fatalf("Write hook1 failed: %v", err)
	}

	read1, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read hook1 failed: %v", err)
	}
	if read1.Seq != 1 {
		t.Errorf("expected first hook seq to be 1, got %d", read1.Seq)
	}

	// Write second hook - seq should increment
	hook2 := &Hook{
		Type:      HookTypeAssign,
		BeadID:    "bd-0002",
		Timestamp: time.Now(),
		Seq:       0, // Will be auto-incremented
	}

	if err := mgr.Write(hook2); err != nil {
		t.Fatalf("Write hook2 failed: %v", err)
	}

	read2, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read hook2 failed: %v", err)
	}
	if read2.Seq != 2 {
		t.Errorf("expected second hook seq to be 2, got %d", read2.Seq)
	}

	// Write third hook with explicit seq - should still auto-increment from previous
	hook3 := &Hook{
		Type:      HookTypeNudge,
		Timestamp: time.Now(),
		Seq:       100, // This should be ignored, will auto-increment to 3
	}

	if err := mgr.Write(hook3); err != nil {
		t.Fatalf("Write hook3 failed: %v", err)
	}

	read3, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read hook3 failed: %v", err)
	}
	if read3.Seq != 3 {
		t.Errorf("expected third hook seq to be 3, got %d", read3.Seq)
	}
}

func TestManager_ClearNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Clear without a hook file existing should not error
	if err := mgr.Clear(); err != nil {
		t.Errorf("Clear should not error on missing file, got: %v", err)
	}
}

func TestHookTypes(t *testing.T) {
	// Verify all hook types have correct string values
	tests := []struct {
		hookType HookType
		expected string
	}{
		{HookTypeAssign, "assign"},
		{HookTypeNudge, "nudge"},
		{HookTypeAbort, "abort"},
		{HookTypePause, "pause"},
		{HookTypeResume, "resume"},
	}

	for _, tt := range tests {
		t.Run(string(tt.hookType), func(t *testing.T) {
			if string(tt.hookType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.hookType))
			}

			// Verify it marshals correctly
			hook := &Hook{Type: tt.hookType, Timestamp: time.Now(), Seq: 1}
			data, err := json.Marshal(hook)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded map[string]interface{}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if decoded["type"] != tt.expected {
				t.Errorf("expected type %s in JSON, got %v", tt.expected, decoded["type"])
			}
		})
	}
}

func TestManager_Watch(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hookChan, err := mgr.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Write a hook after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		hook := &Hook{
			Type:      HookTypeAssign,
			BeadID:    "bd-watch",
			Message:   "Watch test",
			Timestamp: time.Now(),
		}
		if err := mgr.Write(hook); err != nil {
			t.Errorf("Write in goroutine failed: %v", err)
		}
	}()

	// Wait for the hook
	select {
	case receivedHook := <-hookChan:
		if receivedHook == nil {
			t.Error("received nil hook")
		} else if receivedHook.BeadID != "bd-watch" {
			t.Errorf("expected bead_id 'bd-watch', got %v", receivedHook.BeadID)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for hook")
	}
}

func TestManager_WatchCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	hookChan, err := mgr.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Cancel the context
	cancel()

	// Channel should close
	select {
	case _, ok := <-hookChan:
		if ok {
			// Received a value, try again
			select {
			case _, ok := <-hookChan:
				if ok {
					t.Error("expected channel to be closed after context cancellation")
				}
			case <-time.After(500 * time.Millisecond):
				t.Error("channel did not close after context cancellation")
			}
		}
		// Channel closed, test passed
	case <-time.After(500 * time.Millisecond):
		t.Error("channel did not close after context cancellation")
	}
}

func TestNewManager_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	soldatiDir := filepath.Join(tmpDir, "vinnie")

	// Directory shouldn't exist yet
	if _, err := os.Stat(soldatiDir); !os.IsNotExist(err) {
		t.Fatalf("expected directory to not exist initially")
	}

	// Create manager should create the directory
	_, err := NewManager(tmpDir, "vinnie")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(soldatiDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}
