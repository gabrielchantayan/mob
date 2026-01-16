package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// WritePID writes the process ID to a file
func WritePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

// ReadPID reads the process ID from a file
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePID removes the PID file
func RemovePID(path string) error {
	return os.Remove(path)
}

// IsProcessRunning checks if a process with the given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CheckExistingDaemon checks if a daemon is already running
func CheckExistingDaemon(pidFile string) (bool, int, error) {
	pid, err := ReadPID(pidFile)
	if os.IsNotExist(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	if IsProcessRunning(pid) {
		return true, pid, nil
	}

	// Stale PID file, remove it
	RemovePID(pidFile)
	return false, 0, nil
}
