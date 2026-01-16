package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// HookType represents the type of hook message
type HookType string

const (
	HookTypeAssign HookType = "assign" // New work assignment
	HookTypeNudge  HookType = "nudge"  // Wake up signal
	HookTypeAbort  HookType = "abort"  // Cancel current work
	HookTypePause  HookType = "pause"  // Pause execution
	HookTypeResume HookType = "resume" // Resume execution
)

// hookFileName is the standard name for hook files
const hookFileName = "hook.json"

// Hook represents a hook file message
type Hook struct {
	Type      HookType  `json:"type"`
	BeadID    string    `json:"bead_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Seq       int       `json:"seq"` // Sequence number to detect changes
}

// Manager handles hook file operations for a soldati
type Manager struct {
	dir  string // Hook directory (e.g., ~/mob/.mob/soldati/vinnie/)
	name string // Soldati name
	mu   sync.Mutex
	seq  int // Current sequence number
}

// NewManager creates a hook manager for a soldati
func NewManager(baseDir string, name string) (*Manager, error) {
	dir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hook directory: %w", err)
	}

	mgr := &Manager{
		dir:  dir,
		name: name,
		seq:  0,
	}

	// Initialize seq from existing hook file if present
	if hook, err := mgr.Read(); err == nil && hook != nil {
		mgr.seq = hook.Seq
	}

	return mgr, nil
}

// Write writes a new hook message
func (m *Manager) Write(hook *Hook) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Auto-increment sequence number
	m.seq++
	hook.Seq = m.seq

	data, err := json.MarshalIndent(hook, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hook: %w", err)
	}

	hookPath := filepath.Join(m.dir, hookFileName)

	// Write to temp file first for atomic update
	tmpPath := hookPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write hook file: %w", err)
	}

	// Atomically rename to final path
	if err := os.Rename(tmpPath, hookPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename hook file: %w", err)
	}

	return nil
}

// Read reads the current hook file
func (m *Manager) Read() (*Hook, error) {
	hookPath := filepath.Join(m.dir, hookFileName)

	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No hook file is not an error
		}
		return nil, fmt.Errorf("failed to read hook file: %w", err)
	}

	var hook Hook
	if err := json.Unmarshal(data, &hook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hook: %w", err)
	}

	return &hook, nil
}

// Clear removes the hook file (after processing)
func (m *Manager) Clear() error {
	hookPath := filepath.Join(m.dir, hookFileName)

	if err := os.Remove(hookPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleared is not an error
		}
		return fmt.Errorf("failed to clear hook file: %w", err)
	}

	return nil
}

// Watch returns a channel that receives hooks when the file changes
// Uses fsnotify for file system watching
func (m *Manager) Watch(ctx context.Context) (<-chan *Hook, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the directory (not the file itself, since the file may not exist yet)
	if err := watcher.Add(m.dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	hookChan := make(chan *Hook, 1)

	go func() {
		defer watcher.Close()
		defer close(hookChan)

		var lastSeq int

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Check if this is our hook file being written or created
				if filepath.Base(event.Name) == hookFileName {
					if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
						hook, err := m.Read()
						if err != nil || hook == nil {
							continue
						}
						// Only send if seq has changed
						if hook.Seq != lastSeq {
							lastSeq = hook.Seq
							select {
							case hookChan <- hook:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Log errors but continue watching
			}
		}
	}()

	return hookChan, nil
}

// Path returns the full path to the hook file
func (m *Manager) Path() string {
	return filepath.Join(m.dir, hookFileName)
}
