package daemon

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestPIDFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-daemon-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	pidFile := filepath.Join(tmpDir, "daemon.pid")

	// Write PID
	if err := WritePID(pidFile, 12345); err != nil {
		t.Fatalf("failed to write PID: %v", err)
	}

	// Read PID
	pid, err := ReadPID(pidFile)
	if err != nil {
		t.Fatalf("failed to read PID: %v", err)
	}
	if pid != 12345 {
		t.Errorf("expected PID 12345, got %d", pid)
	}

	// Remove PID
	if err := RemovePID(pidFile); err != nil {
		t.Fatalf("failed to remove PID: %v", err)
	}

	// Verify removed
	_, err = ReadPID(pidFile)
	if err == nil {
		t.Error("expected error reading removed PID file")
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Current process should be running
	if !IsProcessRunning(os.Getpid()) {
		t.Error("current process should be running")
	}

	// Non-existent process should not be running
	// Using a very high PID that's unlikely to exist
	if IsProcessRunning(999999999) {
		t.Error("non-existent process should not be running")
	}
}

func TestCheckExistingDaemon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-daemon-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	pidFile := filepath.Join(tmpDir, "daemon.pid")

	// No PID file - should return not running
	running, pid, err := CheckExistingDaemon(pidFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected daemon not running with no PID file")
	}
	if pid != 0 {
		t.Errorf("expected PID 0, got %d", pid)
	}

	// Write current process PID - should return running
	if err := WritePID(pidFile, os.Getpid()); err != nil {
		t.Fatalf("failed to write PID: %v", err)
	}

	running, pid, err = CheckExistingDaemon(pidFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Error("expected daemon running with current process PID")
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}

	// Write stale PID - should return not running and remove file
	if err := WritePID(pidFile, 999999999); err != nil {
		t.Fatalf("failed to write PID: %v", err)
	}

	running, _, err = CheckExistingDaemon(pidFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected daemon not running with stale PID")
	}

	// Verify stale PID file was removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected stale PID file to be removed")
	}
}

func TestDaemonNew(t *testing.T) {
	d := New("/test/mob", log.New(io.Discard, "", 0))

	if d.pidFile != "/test/mob/.mob/daemon.pid" {
		t.Errorf("unexpected pidFile: %s", d.pidFile)
	}
	if d.stateFile != "/test/mob/.mob/daemon.state" {
		t.Errorf("unexpected stateFile: %s", d.stateFile)
	}
	if d.state != StateIdle {
		t.Errorf("expected state Idle, got %s", d.state)
	}
}

func TestDaemonStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-daemon-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .mob directory
	mobDir := filepath.Join(tmpDir, ".mob")
	if err := os.MkdirAll(mobDir, 0755); err != nil {
		t.Fatal(err)
	}

	d := New(tmpDir, log.New(io.Discard, "", 0))

	// No daemon running
	state, pid, err := d.Status()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateIdle {
		t.Errorf("expected state Idle, got %s", state)
	}
	if pid != 0 {
		t.Errorf("expected PID 0, got %d", pid)
	}
}
