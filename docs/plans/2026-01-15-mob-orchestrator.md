# Mob Agent Orchestrator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a mafia-themed Claude Code agent orchestrator enabling autonomous multi-agent workflows with persistent worker identities, structured work tracking, and a hybrid CLI/TUI interface.

**Architecture:** Single Go binary (`mob`) handles CLI commands, TUI dashboard, and daemon mode. The daemon spawns/manages Claude Code instances via JSON-RPC over stdio. Work tracked as JSONL "Beads" with git-backed persistence. File-based hooks coordinate agent work assignments.

**Tech Stack:** Go 1.21+, Cobra (CLI), Bubbletea (TUI), TOML (config), JSONL (beads), JSON-RPC (IPC)

---

## Phase 1: Project Foundation

### Task 1.1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `main.go`

**Step 1: Initialize Go module**

Run:
```bash
cd /Users/gabe/Documents/Programming/mob
go mod init github.com/gabe/mob
```

Expected: Creates `go.mod` with module path

**Step 2: Create minimal main.go**

Create `main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/gabe/mob/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Create cmd package stub**

Create `cmd/root.go`:
```go
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mob",
	Short: "Mob - Claude Code Agent Orchestrator",
	Long:  `A mafia-themed agent orchestrator for managing multiple Claude Code instances.`,
}

func Execute() error {
	return rootCmd.Execute()
}
```

**Step 4: Add Cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
go mod tidy
```

Expected: `go.sum` updated with cobra dependencies

**Step 5: Verify build**

Run:
```bash
go build -o mob .
./mob --help
```

Expected: Shows help text with "Mob - Claude Code Agent Orchestrator"

**Step 6: Commit**

```bash
git add go.mod go.sum main.go cmd/
git commit -m "feat: initialize Go module with Cobra CLI skeleton"
```

---

### Task 1.2: Create Directory Structure

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/models/bead.go`
- Create: `internal/models/soldati.go`
- Create: `internal/models/turf.go`
- Create: `internal/daemon/daemon.go`
- Create: `internal/tui/tui.go`
- Create: `internal/ipc/jsonrpc.go`

**Step 1: Create internal package stubs**

Create `internal/config/config.go`:
```go
package config

// Config holds the main mob configuration
type Config struct {
	Daemon      DaemonConfig      `toml:"daemon"`
	Underboss   UnderbossConfig   `toml:"underboss"`
	Soldati     SoldatiConfig     `toml:"soldati"`
	Associates  AssociatesConfig  `toml:"associates"`
	Notifications NotificationsConfig `toml:"notifications"`
	Safety      SafetyConfig      `toml:"safety"`
	Logging     LoggingConfig     `toml:"logging"`
}

type DaemonConfig struct {
	HeartbeatInterval string `toml:"heartbeat_interval"`
	BootCheckInterval string `toml:"boot_check_interval"`
	StuckTimeout      string `toml:"stuck_timeout"`
	MaxConcurrentAgents int  `toml:"max_concurrent_agents"`
}

type UnderbossConfig struct {
	Personality      string `toml:"personality"`
	ApprovalRequired bool   `toml:"approval_required"`
	HistoryMode      string `toml:"history_mode"`
}

type SoldatiConfig struct {
	AutoName       bool   `toml:"auto_name"`
	DefaultTimeout string `toml:"default_timeout"`
}

type AssociatesConfig struct {
	Timeout      string `toml:"timeout"`
	MaxPerSoldati int   `toml:"max_per_soldati"`
}

type NotificationsConfig struct {
	Terminal        bool   `toml:"terminal"`
	SummaryInterval string `toml:"summary_interval"`
}

type SafetyConfig struct {
	BranchPrefix     string   `toml:"branch_prefix"`
	CommandBlacklist []string `toml:"command_blacklist"`
	RequireReview    bool     `toml:"require_review"`
}

type LoggingConfig struct {
	Level     string `toml:"level"`
	Format    string `toml:"format"`
	Retention string `toml:"retention"`
}
```

**Step 2: Create bead model**

Create `internal/models/bead.go`:
```go
package models

import "time"

// BeadStatus represents the status of a bead
type BeadStatus string

const (
	BeadStatusOpen       BeadStatus = "open"
	BeadStatusInProgress BeadStatus = "in_progress"
	BeadStatusBlocked    BeadStatus = "blocked"
	BeadStatusClosed     BeadStatus = "closed"
	BeadStatusPendingApproval BeadStatus = "pending_approval"
)

// BeadType represents the type of work
type BeadType string

const (
	BeadTypeBug     BeadType = "bug"
	BeadTypeFeature BeadType = "feature"
	BeadTypeTask    BeadType = "task"
	BeadTypeEpic    BeadType = "epic"
	BeadTypeChore   BeadType = "chore"
	BeadTypeReview  BeadType = "review"
	BeadTypeHeresy  BeadType = "heresy"
)

// Bead represents an atomic unit of work
type Bead struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         BeadStatus `json:"status"`
	Priority       int        `json:"priority"` // 0-4, 0 = highest
	Type           BeadType   `json:"type"`
	Assignee       string     `json:"assignee,omitempty"`
	Labels         string     `json:"labels,omitempty"`
	Turf           string     `json:"turf"`
	Branch         string     `json:"branch,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	CreatedBy      string     `json:"created_by,omitempty"`
	CloseReason    string     `json:"close_reason,omitempty"`
	ParentID       string     `json:"parent_id,omitempty"`
	Blocks         []string   `json:"blocks,omitempty"`
	Related        []string   `json:"related,omitempty"`
	DiscoveredFrom string     `json:"discovered_from,omitempty"`
}
```

**Step 3: Create soldati model**

Create `internal/models/soldati.go`:
```go
package models

import "time"

// SoldatiStats tracks performance metrics
type SoldatiStats struct {
	TasksCompleted int     `toml:"tasks_completed"`
	TasksFailed    int     `toml:"tasks_failed"`
	SuccessRate    float64 `toml:"success_rate"`
}

// Soldati represents a named, persistent worker
type Soldati struct {
	Name       string       `toml:"name"`
	CreatedAt  time.Time    `toml:"created_at"`
	LastActive time.Time    `toml:"last_active"`
	Stats      SoldatiStats `toml:"stats"`
}
```

**Step 4: Create turf model**

Create `internal/models/turf.go`:
```go
package models

// Turf represents a registered project
type Turf struct {
	Name       string `toml:"name"`
	Path       string `toml:"path"`
	MainBranch string `toml:"main_branch"`
}

// TurfsConfig holds all registered turfs
type TurfsConfig struct {
	Turfs []Turf `toml:"turf"`
}
```

**Step 5: Create daemon stub**

Create `internal/daemon/daemon.go`:
```go
package daemon

// Daemon manages the mob orchestration
type Daemon struct {
	pidFile   string
	stateFile string
	running   bool
}

// New creates a new daemon instance
func New(pidFile, stateFile string) *Daemon {
	return &Daemon{
		pidFile:   pidFile,
		stateFile: stateFile,
	}
}
```

**Step 6: Create TUI stub**

Create `internal/tui/tui.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

// Model represents the TUI state
type Model struct {
	activeTab int
}

// New creates a new TUI model
func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return "Mob TUI - Coming Soon"
}
```

**Step 7: Create IPC stub**

Create `internal/ipc/jsonrpc.go`:
```go
package ipc

// Client handles JSON-RPC communication with Claude Code
type Client struct {
	// Will hold stdin/stdout pipes
}

// New creates a new IPC client
func New() *Client {
	return &Client{}
}
```

**Step 8: Add dependencies and verify**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/BurntSushi/toml@latest
go mod tidy
go build ./...
```

Expected: Build succeeds with no errors

**Step 9: Commit**

```bash
git add internal/
git commit -m "feat: add internal package structure with models and stubs"
```

---

### Task 1.3: Add Version Command

**Files:**
- Modify: `cmd/root.go`
- Create: `cmd/version.go`
- Create: `internal/version/version.go`

**Step 1: Create version package**

Create `internal/version/version.go`:
```go
package version

var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)
```

**Step 2: Create version command**

Create `cmd/version.go`:
```go
package cmd

import (
	"fmt"

	"github.com/gabe/mob/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mob version %s\n", version.Version)
		fmt.Printf("  commit: %s\n", version.GitCommit)
		fmt.Printf("  built:  %s\n", version.BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

**Step 3: Verify version command**

Run:
```bash
go build -o mob .
./mob version
```

Expected:
```
mob version 0.1.0
  commit: unknown
  built:  unknown
```

**Step 4: Commit**

```bash
git add cmd/version.go internal/version/
git commit -m "feat: add version command"
```

---

## Phase 2: Configuration & Data Layer

### Task 2.1: Implement Config Loading

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/loader.go`
- Create: `internal/config/defaults.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing test for config loading**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mob-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write test config
	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
[daemon]
heartbeat_interval = "3m"
max_concurrent_agents = 10

[safety]
branch_prefix = "mob/"
command_blacklist = ["sudo", "rm -rf"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values
	if cfg.Daemon.HeartbeatInterval != "3m" {
		t.Errorf("expected heartbeat_interval '3m', got '%s'", cfg.Daemon.HeartbeatInterval)
	}
	if cfg.Daemon.MaxConcurrentAgents != 10 {
		t.Errorf("expected max_concurrent_agents 10, got %d", cfg.Daemon.MaxConcurrentAgents)
	}
	if cfg.Safety.BranchPrefix != "mob/" {
		t.Errorf("expected branch_prefix 'mob/', got '%s'", cfg.Safety.BranchPrefix)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create temp directory with empty config
	tmpDir, err := os.MkdirTemp("", "mob-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Should have defaults
	if cfg.Daemon.HeartbeatInterval != "2m" {
		t.Errorf("expected default heartbeat_interval '2m', got '%s'", cfg.Daemon.HeartbeatInterval)
	}
	if cfg.Daemon.MaxConcurrentAgents != 5 {
		t.Errorf("expected default max_concurrent_agents 5, got %d", cfg.Daemon.MaxConcurrentAgents)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/... -v
```

Expected: FAIL - `Load` function not defined

**Step 3: Create defaults**

Create `internal/config/defaults.go`:
```go
package config

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			HeartbeatInterval:   "2m",
			BootCheckInterval:   "5m",
			StuckTimeout:        "10m",
			MaxConcurrentAgents: 5,
		},
		Underboss: UnderbossConfig{
			Personality:      "efficient mob underboss",
			ApprovalRequired: true,
			HistoryMode:      "hybrid",
		},
		Soldati: SoldatiConfig{
			AutoName:       true,
			DefaultTimeout: "30m",
		},
		Associates: AssociatesConfig{
			Timeout:       "10m",
			MaxPerSoldati: 3,
		},
		Notifications: NotificationsConfig{
			Terminal:        true,
			SummaryInterval: "1h",
		},
		Safety: SafetyConfig{
			BranchPrefix:     "mob/",
			CommandBlacklist: []string{"sudo", "rm -rf"},
			RequireReview:    true,
		},
		Logging: LoggingConfig{
			Level:     "info",
			Format:    "dual",
			Retention: "7d",
		},
	}
}
```

**Step 4: Create loader**

Create `internal/config/loader.go`:
```go
package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// Load reads config from path, applying defaults for missing values
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadOrCreate loads config or creates default if missing
func LoadOrCreate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		return cfg, Save(path, cfg)
	}
	return Load(path)
}

// Save writes config to path
func Save(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}
```

**Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/... -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat: implement config loading with TOML and defaults"
```

---

### Task 2.2: Implement Bead Storage

**Files:**
- Create: `internal/storage/bead_store.go`
- Create: `internal/storage/bead_store_test.go`

**Step 1: Write failing test for bead storage**

Create `internal/storage/bead_store_test.go`:
```go
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabe/mob/internal/models"
)

func TestBeadStore_Create(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	bead := &models.Bead{
		Title:       "Test task",
		Description: "A test task",
		Status:      models.BeadStatusOpen,
		Priority:    1,
		Type:        models.BeadTypeTask,
		Turf:        "test-project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := store.Create(bead)
	if err != nil {
		t.Fatalf("failed to create bead: %v", err)
	}

	if created.ID == "" {
		t.Error("expected bead to have ID")
	}
	if created.Title != "Test task" {
		t.Errorf("expected title 'Test task', got '%s'", created.Title)
	}
}

func TestBeadStore_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a few beads
	for i := 0; i < 3; i++ {
		bead := &models.Bead{
			Title:     "Task",
			Status:    models.BeadStatusOpen,
			Type:      models.BeadTypeTask,
			Turf:      "test",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if _, err := store.Create(bead); err != nil {
			t.Fatal(err)
		}
	}

	beads, err := store.List(BeadFilter{})
	if err != nil {
		t.Fatalf("failed to list beads: %v", err)
	}

	if len(beads) != 3 {
		t.Errorf("expected 3 beads, got %d", len(beads))
	}
}

func TestBeadStore_Update(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-bead-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewBeadStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	bead := &models.Bead{
		Title:     "Original",
		Status:    models.BeadStatusOpen,
		Type:      models.BeadTypeTask,
		Turf:      "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	created, err := store.Create(bead)
	if err != nil {
		t.Fatal(err)
	}

	created.Title = "Updated"
	created.Status = models.BeadStatusInProgress
	updated, err := store.Update(created)
	if err != nil {
		t.Fatalf("failed to update bead: %v", err)
	}

	if updated.Title != "Updated" {
		t.Errorf("expected title 'Updated', got '%s'", updated.Title)
	}
	if updated.Status != models.BeadStatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", updated.Status)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/storage/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Implement bead storage**

Create `internal/storage/bead_store.go`:
```go
package storage

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gabe/mob/internal/models"
)

// BeadStore manages JSONL-based bead storage
type BeadStore struct {
	dir      string
	openFile string
	mu       sync.RWMutex
}

// BeadFilter defines filtering options for listing beads
type BeadFilter struct {
	Status   models.BeadStatus
	Turf     string
	Assignee string
	Type     models.BeadType
}

// NewBeadStore creates a new bead store at the given directory
func NewBeadStore(dir string) (*BeadStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bead directory: %w", err)
	}

	return &BeadStore{
		dir:      dir,
		openFile: filepath.Join(dir, "open.jsonl"),
	}, nil
}

// generateID creates a short random ID for beads
func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "bd-" + hex.EncodeToString(b)[:4]
}

// Create adds a new bead to the store
func (s *BeadStore) Create(bead *models.Bead) (*models.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bead.ID = generateID()
	bead.CreatedAt = time.Now()
	bead.UpdatedAt = time.Now()
	bead.Branch = "mob/" + bead.ID

	return bead, s.appendBead(bead)
}

// List returns all beads matching the filter
func (s *BeadStore) List(filter BeadFilter) ([]*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	// Apply filters
	var filtered []*models.Bead
	for _, bead := range beads {
		if filter.Status != "" && bead.Status != filter.Status {
			continue
		}
		if filter.Turf != "" && bead.Turf != filter.Turf {
			continue
		}
		if filter.Assignee != "" && bead.Assignee != filter.Assignee {
			continue
		}
		if filter.Type != "" && bead.Type != filter.Type {
			continue
		}
		filtered = append(filtered, bead)
	}

	return filtered, nil
}

// Get retrieves a bead by ID
func (s *BeadStore) Get(id string) (*models.Bead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	for _, bead := range beads {
		if bead.ID == id {
			return bead, nil
		}
	}

	return nil, fmt.Errorf("bead not found: %s", id)
}

// Update modifies an existing bead
func (s *BeadStore) Update(bead *models.Bead) (*models.Bead, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	beads, err := s.readAllBeads()
	if err != nil {
		return nil, err
	}

	found := false
	for i, b := range beads {
		if b.ID == bead.ID {
			bead.UpdatedAt = time.Now()
			beads[i] = bead
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("bead not found: %s", bead.ID)
	}

	return bead, s.writeAllBeads(beads)
}

func (s *BeadStore) appendBead(bead *models.Bead) error {
	f, err := os.OpenFile(s.openFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(bead)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *BeadStore) readAllBeads() ([]*models.Bead, error) {
	f, err := os.Open(s.openFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var beads []*models.Bead
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var bead models.Bead
		if err := json.Unmarshal(scanner.Bytes(), &bead); err != nil {
			continue // Skip malformed lines
		}
		beads = append(beads, &bead)
	}

	return beads, scanner.Err()
}

func (s *BeadStore) writeAllBeads(beads []*models.Bead) error {
	// Write to temp file first
	tmpFile := s.openFile + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	for _, bead := range beads {
		data, err := json.Marshal(bead)
		if err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return os.Rename(tmpFile, s.openFile)
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/storage/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/
git commit -m "feat: implement JSONL-based bead storage"
```

---

### Task 2.3: Implement Turf Management

**Files:**
- Create: `internal/turf/manager.go`
- Create: `internal/turf/manager_test.go`

**Step 1: Write failing test for turf manager**

Create `internal/turf/manager_test.go`:
```go
package turf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTurfManager_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake project directory
	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = mgr.Add(projectDir, "my-project", "main")
	if err != nil {
		t.Fatalf("failed to add turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 1 {
		t.Errorf("expected 1 turf, got %d", len(turfs))
	}
	if turfs[0].Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", turfs[0].Name)
	}
}

func TestTurfManager_Remove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-turf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	turfsFile := filepath.Join(tmpDir, "turfs.toml")
	mgr, err := NewManager(turfsFile)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Add(projectDir, "my-project", "main")
	err = mgr.Remove("my-project")
	if err != nil {
		t.Fatalf("failed to remove turf: %v", err)
	}

	turfs := mgr.List()
	if len(turfs) != 0 {
		t.Errorf("expected 0 turfs, got %d", len(turfs))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/turf/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Implement turf manager**

Create `internal/turf/manager.go`:
```go
package turf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gabe/mob/internal/models"
)

// Manager handles turf registration and lookup
type Manager struct {
	path   string
	config models.TurfsConfig
}

// NewManager creates a new turf manager
func NewManager(path string) (*Manager, error) {
	mgr := &Manager{path: path}

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read turfs file: %w", err)
		}
		if _, err := toml.Decode(string(data), &mgr.config); err != nil {
			return nil, fmt.Errorf("failed to parse turfs file: %w", err)
		}
	}

	return mgr, nil
}

// Add registers a new turf
func (m *Manager) Add(path, name, mainBranch string) error {
	// Validate path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check for duplicate
	for _, t := range m.config.Turfs {
		if t.Name == name {
			return fmt.Errorf("turf already exists: %s", name)
		}
		if t.Path == absPath {
			return fmt.Errorf("path already registered as turf: %s", t.Name)
		}
	}

	m.config.Turfs = append(m.config.Turfs, models.Turf{
		Name:       name,
		Path:       absPath,
		MainBranch: mainBranch,
	})

	return m.save()
}

// Remove unregisters a turf
func (m *Manager) Remove(name string) error {
	for i, t := range m.config.Turfs {
		if t.Name == name {
			m.config.Turfs = append(m.config.Turfs[:i], m.config.Turfs[i+1:]...)
			return m.save()
		}
	}
	return fmt.Errorf("turf not found: %s", name)
}

// List returns all registered turfs
func (m *Manager) List() []models.Turf {
	return m.config.Turfs
}

// Get retrieves a turf by name
func (m *Manager) Get(name string) (*models.Turf, error) {
	for _, t := range m.config.Turfs {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("turf not found: %s", name)
}

func (m *Manager) save() error {
	f, err := os.Create(m.path)
	if err != nil {
		return fmt.Errorf("failed to create turfs file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(m.config)
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/turf/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/turf/
git commit -m "feat: implement turf manager for project registration"
```

---

## Phase 3: Core CLI Commands

### Task 3.1: Implement Init Command

**Files:**
- Create: `cmd/init.go`
- Create: `internal/setup/wizard.go`

**Step 1: Create setup wizard**

Create `internal/setup/wizard.go`:
```go
package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabe/mob/internal/config"
)

// Wizard handles interactive first-run setup
type Wizard struct {
	reader *bufio.Reader
}

// NewWizard creates a setup wizard
func NewWizard() *Wizard {
	return &Wizard{
		reader: bufio.NewReader(os.Stdin),
	}
}

// Run executes the setup wizard
func (w *Wizard) Run() error {
	fmt.Println("Welcome to Mob - Claude Code Agent Orchestrator")
	fmt.Println("================================================")
	fmt.Println()

	// Get mob home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultMobDir := filepath.Join(homeDir, "mob")
	mobDir, err := w.prompt("Where should mob store its data?", defaultMobDir)
	if err != nil {
		return err
	}

	// Create directory structure
	dirs := []string{
		mobDir,
		filepath.Join(mobDir, ".mob"),
		filepath.Join(mobDir, ".mob", "logs"),
		filepath.Join(mobDir, ".mob", "logs", "soldati"),
		filepath.Join(mobDir, ".mob", "tmp"),
		filepath.Join(mobDir, ".mob", "soldati"),
		filepath.Join(mobDir, "beads"),
		filepath.Join(mobDir, "beads", "archive"),
		filepath.Join(mobDir, "soldati"),
		filepath.Join(mobDir, "history"),
		filepath.Join(mobDir, "history", "summaries"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default config
	configPath := filepath.Join(mobDir, "config.toml")
	cfg := config.DefaultConfig()
	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create empty turfs file
	turfsPath := filepath.Join(mobDir, "turfs.toml")
	if err := os.WriteFile(turfsPath, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create turfs file: %w", err)
	}

	fmt.Println()
	fmt.Println("Setup complete!")
	fmt.Printf("  Mob home: %s\n", mobDir)
	fmt.Printf("  Config:   %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Register a project: mob turf add /path/to/project")
	fmt.Println("  2. Start the daemon:   mob daemon start")
	fmt.Println("  3. Chat with mob:      mob chat")

	return nil
}

func (w *Wizard) prompt(question, defaultVal string) (string, error) {
	fmt.Printf("%s [%s]: ", question, defaultVal)
	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}
```

**Step 2: Create init command**

Create `cmd/init.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/setup"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mob with interactive setup",
	Long:  `Run the first-time setup wizard to configure mob directories and settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		wizard := setup.NewWizard()
		if err := wizard.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

**Step 3: Verify init command**

Run:
```bash
go build -o mob .
./mob init --help
```

Expected: Shows help for init command

**Step 4: Commit**

```bash
git add cmd/init.go internal/setup/
git commit -m "feat: add init command with setup wizard"
```

---

### Task 3.2: Implement Turf Commands

**Files:**
- Create: `cmd/turf.go`
- Modify: `internal/turf/manager.go` (add path resolution)

**Step 1: Create turf command group**

Create `cmd/turf.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gabe/mob/internal/turf"
	"github.com/spf13/cobra"
)

var turfCmd = &cobra.Command{
	Use:   "turf",
	Short: "Manage registered projects (turfs)",
	Long:  `Register, list, and remove projects under mob's management.`,
}

var turfAddCmd = &cobra.Command{
	Use:   "add <path> [name]",
	Short: "Register a project as a turf",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		name := ""
		if len(args) > 1 {
			name = args[1]
		} else {
			// Use directory name as default
			name = filepath.Base(path)
		}

		mainBranch, _ := cmd.Flags().GetString("branch")

		turfsPath := getTurfsPath()
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Add(path, name, mainBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registered turf '%s' at %s\n", name, path)
	},
}

var turfListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered turfs",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		turfsPath := getTurfsPath()
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		turfs := mgr.List()
		if len(turfs) == 0 {
			fmt.Println("No turfs registered. Use 'mob turf add <path>' to register a project.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPATH\tBRANCH")
		for _, t := range turfs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.Path, t.MainBranch)
		}
		w.Flush()
	},
}

var turfRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a turf",
	Aliases: []string{"rm"},
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		turfsPath := getTurfsPath()
		mgr, err := turf.NewManager(turfsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Remove(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed turf '%s'\n", name)
	},
}

func getTurfsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob", "turfs.toml")
}

func init() {
	turfAddCmd.Flags().StringP("branch", "b", "main", "Main branch name")

	turfCmd.AddCommand(turfAddCmd)
	turfCmd.AddCommand(turfListCmd)
	turfCmd.AddCommand(turfRemoveCmd)
	rootCmd.AddCommand(turfCmd)
}
```

**Step 2: Verify turf commands**

Run:
```bash
go build -o mob .
./mob turf --help
./mob turf add --help
./mob turf list --help
```

Expected: Shows help for all turf subcommands

**Step 3: Commit**

```bash
git add cmd/turf.go
git commit -m "feat: add turf management commands"
```

---

### Task 3.3: Implement Add/Status Commands for Beads

**Files:**
- Create: `cmd/add.go`
- Create: `cmd/status.go`

**Step 1: Create add command**

Create `cmd/add.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <description>",
	Short: "Create a new bead (task)",
	Long:  `Create a new bead with the given description. The bead will be added to the open queue.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		description := strings.Join(args, " ")

		priority, _ := cmd.Flags().GetInt("priority")
		beadType, _ := cmd.Flags().GetString("type")
		turfName, _ := cmd.Flags().GetString("turf")
		labels, _ := cmd.Flags().GetString("labels")

		beadsPath := getBeadsPath()
		store, err := storage.NewBeadStore(beadsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		bead := &models.Bead{
			Title:       description,
			Description: description,
			Status:      models.BeadStatusOpen,
			Priority:    priority,
			Type:        models.BeadType(beadType),
			Turf:        turfName,
			Labels:      labels,
		}

		created, err := store.Create(bead)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created bead %s: %s\n", created.ID, created.Title)
	},
}

func getBeadsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob", "beads")
}

func init() {
	addCmd.Flags().IntP("priority", "p", 2, "Priority (0=highest, 4=lowest)")
	addCmd.Flags().StringP("type", "t", "task", "Type (bug, feature, task, chore)")
	addCmd.Flags().String("turf", "", "Target turf")
	addCmd.Flags().StringP("labels", "l", "", "Comma-separated labels")

	rootCmd.AddCommand(addCmd)
}
```

**Step 2: Create status command**

Create `cmd/status.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [bead-id]",
	Short: "Show status of beads",
	Long:  `Show status of all beads or a specific bead by ID.`,
	Aliases: []string{"s"},
	Run: func(cmd *cobra.Command, args []string) {
		beadsPath := getBeadsPath()
		store, err := storage.NewBeadStore(beadsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(args) > 0 {
			// Show specific bead
			bead, err := store.Get(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			printBeadDetail(bead)
			return
		}

		// Show all beads
		filterStatus, _ := cmd.Flags().GetString("status")
		filterTurf, _ := cmd.Flags().GetString("turf")

		filter := storage.BeadFilter{
			Status: models.BeadStatus(filterStatus),
			Turf:   filterTurf,
		}

		beads, err := store.List(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(beads) == 0 {
			fmt.Println("No beads found. Use 'mob add <description>' to create one.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tPRI\tTYPE\tTITLE\tTURF")
		for _, b := range beads {
			title := b.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
				b.ID, b.Status, b.Priority, b.Type, title, b.Turf)
		}
		w.Flush()
	},
}

func printBeadDetail(b *models.Bead) {
	fmt.Printf("Bead: %s\n", b.ID)
	fmt.Printf("  Title:       %s\n", b.Title)
	fmt.Printf("  Status:      %s\n", b.Status)
	fmt.Printf("  Priority:    %d\n", b.Priority)
	fmt.Printf("  Type:        %s\n", b.Type)
	if b.Turf != "" {
		fmt.Printf("  Turf:        %s\n", b.Turf)
	}
	if b.Assignee != "" {
		fmt.Printf("  Assignee:    %s\n", b.Assignee)
	}
	if b.Labels != "" {
		fmt.Printf("  Labels:      %s\n", b.Labels)
	}
	if b.Branch != "" {
		fmt.Printf("  Branch:      %s\n", b.Branch)
	}
	fmt.Printf("  Created:     %s\n", b.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:     %s\n", b.UpdatedAt.Format(time.RFC3339))
	if b.Description != b.Title {
		fmt.Printf("\nDescription:\n%s\n", b.Description)
	}
}

func init() {
	statusCmd.Flags().String("status", "", "Filter by status")
	statusCmd.Flags().String("turf", "", "Filter by turf")

	rootCmd.AddCommand(statusCmd)
}
```

**Step 3: Verify commands**

Run:
```bash
go build -o mob .
./mob add --help
./mob status --help
```

Expected: Shows help for both commands

**Step 4: Commit**

```bash
git add cmd/add.go cmd/status.go
git commit -m "feat: add bead creation and status commands"
```

---

## Phase 4: Daemon Foundation

### Task 4.1: Implement Daemon Process Management

**Files:**
- Modify: `internal/daemon/daemon.go`
- Create: `internal/daemon/pid.go`
- Create: `internal/daemon/daemon_test.go`
- Create: `cmd/daemon.go`

**Step 1: Write failing test for PID management**

Create `internal/daemon/daemon_test.go`:
```go
package daemon

import (
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
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/daemon/... -v
```

Expected: FAIL - functions not defined

**Step 3: Implement PID management**

Create `internal/daemon/pid.go`:
```go
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
```

**Step 4: Update daemon.go with full implementation**

Update `internal/daemon/daemon.go`:
```go
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// State represents the daemon's operational state
type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StatePaused  State = "paused"
)

// Daemon manages the mob orchestration
type Daemon struct {
	pidFile   string
	stateFile string
	state     State
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new daemon instance
func New(mobDir string) *Daemon {
	return &Daemon{
		pidFile:   filepath.Join(mobDir, ".mob", "daemon.pid"),
		stateFile: filepath.Join(mobDir, ".mob", "daemon.state"),
		state:     StateIdle,
	}
}

// Start begins daemon operation
func (d *Daemon) Start() error {
	// Check for existing daemon
	running, pid, err := CheckExistingDaemon(d.pidFile)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	// Write our PID
	if err := WritePID(d.pidFile, os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Set up context for graceful shutdown
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.state = StateRunning

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Mob daemon started")

	// Main loop
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.shutdown()
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			return d.shutdown()
		case <-ticker.C:
			d.patrol()
		}
	}
}

// Stop gracefully stops the daemon
func (d *Daemon) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}
	return nil
}

// Status returns the current daemon status
func (d *Daemon) Status() (State, int, error) {
	running, pid, err := CheckExistingDaemon(d.pidFile)
	if err != nil {
		return "", 0, err
	}
	if !running {
		return StateIdle, 0, nil
	}
	return StateRunning, pid, nil
}

func (d *Daemon) shutdown() error {
	d.state = StateIdle
	RemovePID(d.pidFile)
	fmt.Println("Mob daemon stopped")
	return nil
}

func (d *Daemon) patrol() {
	// Placeholder for patrol loop logic
	// Will check agent health, handle stuck workers, etc.
}
```

**Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/daemon/... -v
```

Expected: PASS

**Step 6: Create daemon command**

Create `cmd/daemon.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gabe/mob/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the mob daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mob daemon",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir := getMobDir()
		d := daemon.New(mobDir)

		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the mob daemon",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir := getMobDir()
		pidFile := filepath.Join(mobDir, ".mob", "daemon.pid")

		pid, err := daemon.ReadPID(pidFile)
		if os.IsNotExist(err) {
			fmt.Println("Daemon not running")
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding process: %v\n", err)
			os.Exit(1)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping daemon: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Daemon stop signal sent")
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Run: func(cmd *cobra.Command, args []string) {
		mobDir := getMobDir()
		d := daemon.New(mobDir)

		state, pid, err := d.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if state == daemon.StateIdle {
			fmt.Println("Daemon: not running")
		} else {
			fmt.Printf("Daemon: %s (PID %d)\n", state, pid)
		}
	},
}

func getMobDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob")
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
```

**Step 7: Verify daemon commands**

Run:
```bash
go build -o mob .
./mob daemon --help
./mob daemon status
```

Expected: Shows daemon help and status (not running)

**Step 8: Commit**

```bash
git add internal/daemon/ cmd/daemon.go
git commit -m "feat: implement daemon process management"
```

---

## Phase 5: Soldati Management

### Task 5.1: Implement Soldati Storage

**Files:**
- Create: `internal/soldati/manager.go`
- Create: `internal/soldati/manager_test.go`
- Create: `internal/soldati/names.go`

**Step 1: Write failing test for soldati manager**

Create `internal/soldati/manager_test.go`:
```go
package soldati

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSoldatiManager_Create(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-soldati-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	soldati, err := mgr.Create("vinnie")
	if err != nil {
		t.Fatalf("failed to create soldati: %v", err)
	}

	if soldati.Name != "vinnie" {
		t.Errorf("expected name 'vinnie', got '%s'", soldati.Name)
	}

	// Verify file was created
	filePath := filepath.Join(tmpDir, "vinnie.toml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("soldati file was not created")
	}
}

func TestSoldatiManager_CreateAutoName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-soldati-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	soldati, err := mgr.Create("")
	if err != nil {
		t.Fatalf("failed to create soldati: %v", err)
	}

	if soldati.Name == "" {
		t.Error("expected auto-generated name")
	}
}

func TestSoldatiManager_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mob-soldati-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	mgr.Create("vinnie")
	mgr.Create("sal")
	mgr.Create("tony")

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 soldati, got %d", len(list))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/soldati/... -v
```

Expected: FAIL - package doesn't exist

**Step 3: Create mob names generator**

Create `internal/soldati/names.go`:
```go
package soldati

import (
	"math/rand"
	"time"
)

var mobNames = []string{
	"vinnie", "sal", "tony", "joey", "frankie",
	"paulie", "gino", "carmine", "luca", "rocco",
	"enzo", "vito", "sonny", "mikey", "nicky",
	"angelo", "bruno", "carlo", "dante", "aldo",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateName returns a random mob-themed name
func GenerateName() string {
	return mobNames[rand.Intn(len(mobNames))]
}

// GenerateUniqueName returns a name not in the used list
func GenerateUniqueName(used []string) string {
	usedSet := make(map[string]bool)
	for _, name := range used {
		usedSet[name] = true
	}

	// Try random names first
	for i := 0; i < 10; i++ {
		name := GenerateName()
		if !usedSet[name] {
			return name
		}
	}

	// Fallback to sequential search
	for _, name := range mobNames {
		if !usedSet[name] {
			return name
		}
	}

	// All names used, add a number suffix
	base := GenerateName()
	for i := 2; ; i++ {
		name := base + string(rune('0'+i))
		if !usedSet[name] {
			return name
		}
	}
}
```

**Step 4: Implement soldati manager**

Create `internal/soldati/manager.go`:
```go
package soldati

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gabe/mob/internal/models"
)

// Manager handles soldati creation and management
type Manager struct {
	dir string
}

// NewManager creates a new soldati manager
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create soldati directory: %w", err)
	}
	return &Manager{dir: dir}, nil
}

// Create creates a new soldati with the given name (or auto-generates one)
func (m *Manager) Create(name string) (*models.Soldati, error) {
	if name == "" {
		existing, _ := m.listNames()
		name = GenerateUniqueName(existing)
	}

	// Check for existing
	if _, err := m.Get(name); err == nil {
		return nil, fmt.Errorf("soldati already exists: %s", name)
	}

	soldati := &models.Soldati{
		Name:       name,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Stats: models.SoldatiStats{
			TasksCompleted: 0,
			TasksFailed:    0,
			SuccessRate:    0.0,
		},
	}

	if err := m.save(soldati); err != nil {
		return nil, err
	}

	return soldati, nil
}

// Get retrieves a soldati by name
func (m *Manager) Get(name string) (*models.Soldati, error) {
	path := filepath.Join(m.dir, name+".toml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("soldati not found: %s", name)
	}

	var soldati models.Soldati
	if _, err := toml.Decode(string(data), &soldati); err != nil {
		return nil, fmt.Errorf("failed to parse soldati file: %w", err)
	}

	return &soldati, nil
}

// List returns all soldati
func (m *Manager) List() ([]*models.Soldati, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read soldati directory: %w", err)
	}

	var soldati []*models.Soldati
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".toml")
		s, err := m.Get(name)
		if err != nil {
			continue
		}
		soldati = append(soldati, s)
	}

	return soldati, nil
}

// Update saves changes to a soldati
func (m *Manager) Update(soldati *models.Soldati) error {
	soldati.LastActive = time.Now()
	return m.save(soldati)
}

// Delete removes a soldati
func (m *Manager) Delete(name string) error {
	path := filepath.Join(m.dir, name+".toml")
	return os.Remove(path)
}

func (m *Manager) save(soldati *models.Soldati) error {
	path := filepath.Join(m.dir, soldati.Name+".toml")

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create soldati file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(soldati)
}

func (m *Manager) listNames() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".toml"))
	}

	return names, nil
}
```

**Step 5: Run tests to verify they pass**

Run:
```bash
go test ./internal/soldati/... -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add internal/soldati/
git commit -m "feat: implement soldati management with mob-themed auto-naming"
```

---

### Task 5.2: Add Soldati CLI Commands

**Files:**
- Create: `cmd/soldati.go`

**Step 1: Create soldati command group**

Create `cmd/soldati.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/soldati"
	"github.com/spf13/cobra"
)

var soldatiCmd = &cobra.Command{
	Use:   "soldati",
	Short: "Manage worker agents (soldati)",
	Long:  `Create, list, and manage persistent worker agents.`,
}

var soldatiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all soldati",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		soldatiDir := getSoldatiDir()
		mgr, err := soldati.NewManager(soldatiDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		list, err := mgr.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(list) == 0 {
			fmt.Println("No soldati. Use 'mob soldati new' to create one.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTASKS\tSUCCESS\tLAST ACTIVE")
		for _, s := range list {
			successRate := fmt.Sprintf("%.0f%%", s.Stats.SuccessRate*100)
			lastActive := time.Since(s.LastActive).Round(time.Minute).String() + " ago"
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
				s.Name, s.Stats.TasksCompleted, successRate, lastActive)
		}
		w.Flush()
	},
}

var soldatiNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new soldati",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		soldatiDir := getSoldatiDir()
		mgr, err := soldati.NewManager(soldatiDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		s, err := mgr.Create(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created soldati '%s'\n", s.Name)
	},
}

var soldatiKillCmd = &cobra.Command{
	Use:   "kill <name>",
	Short: "Terminate a soldati",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		soldatiDir := getSoldatiDir()
		mgr, err := soldati.NewManager(soldatiDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := mgr.Delete(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Killed soldati '%s'\n", name)
	},
}

func getSoldatiDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mob", "soldati")
}

func init() {
	soldatiCmd.AddCommand(soldatiListCmd)
	soldatiCmd.AddCommand(soldatiNewCmd)
	soldatiCmd.AddCommand(soldatiKillCmd)
	rootCmd.AddCommand(soldatiCmd)
}
```

**Step 2: Verify soldati commands**

Run:
```bash
go build -o mob .
./mob soldati --help
./mob soldati list
```

Expected: Shows help and empty list

**Step 3: Commit**

```bash
git add cmd/soldati.go
git commit -m "feat: add soldati management CLI commands"
```

---

## Phase 6: TUI Foundation

### Task 6.1: Implement Basic TUI Shell

**Files:**
- Modify: `internal/tui/tui.go`
- Create: `internal/tui/styles.go`
- Create: `cmd/tui.go`

**Step 1: Create TUI styles**

Create `internal/tui/styles.go`:
```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#10B981")
	mutedColor     = lipgloss.Color("#6B7280")
	errorColor     = lipgloss.Color("#EF4444")

	// Tab styles
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor)

	// Content styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)
)
```

**Step 2: Update TUI implementation**

Update `internal/tui/tui.go`:
```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabDashboard tab = iota
	tabAgents
	tabBeads
	tabLogs
)

var tabs = []string{"Dashboard", "Agents", "Beads", "Logs"}

// Model represents the TUI state
type Model struct {
	activeTab tab
	width     int
	height    int
	quitting  bool
}

// New creates a new TUI model
func New() Model {
	return Model{
		activeTab: tabDashboard,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % tab(len(tabs))
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tab(len(tabs))) % tab(len(tabs))
		case "1":
			m.activeTab = tabDashboard
		case "2":
			m.activeTab = tabAgents
		case "3":
			m.activeTab = tabBeads
		case "4":
			m.activeTab = tabLogs
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("MOB - Agent Orchestrator"))
	b.WriteString("\n\n")

	// Tab bar
	var tabItems []string
	for i, t := range tabs {
		if tab(i) == m.activeTab {
			tabItems = append(tabItems, activeTabStyle.Render(t))
		} else {
			tabItems = append(tabItems, inactiveTabStyle.Render(t))
		}
	}
	b.WriteString(tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, tabItems...)))
	b.WriteString("\n\n")

	// Content
	switch m.activeTab {
	case tabDashboard:
		b.WriteString(m.renderDashboard())
	case tabAgents:
		b.WriteString(m.renderAgents())
	case tabBeads:
		b.WriteString(m.renderBeads())
	case tabLogs:
		b.WriteString(m.renderLogs())
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab/arrows: switch tabs • 1-4: jump to tab • q: quit"))

	return b.String()
}

func (m Model) renderDashboard() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Dashboard"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Daemon:   %s\n", statusStyle.Render("Not Running")))
	b.WriteString(fmt.Sprintf("Soldati:  %s\n", "0 active"))
	b.WriteString(fmt.Sprintf("Beads:    %s\n", "0 open"))
	b.WriteString(fmt.Sprintf("Turfs:    %s\n", "0 registered"))

	return b.String()
}

func (m Model) renderAgents() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Agents"))
	b.WriteString("\n\n")
	b.WriteString("No active agents.\n")
	b.WriteString("Use 'mob soldati new' to create a worker.\n")
	return b.String()
}

func (m Model) renderBeads() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Beads"))
	b.WriteString("\n\n")
	b.WriteString("No beads.\n")
	b.WriteString("Use 'mob add <task>' to create a bead.\n")
	return b.String()
}

func (m Model) renderLogs() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Logs"))
	b.WriteString("\n\n")
	b.WriteString("No logs yet.\n")
	return b.String()
}

// Run starts the TUI
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

**Step 3: Create TUI command**

Create `cmd/tui.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/gabe/mob/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the TUI dashboard",
	Long:  `Launch an interactive terminal dashboard for monitoring and managing mob.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
```

**Step 4: Add lipgloss dependency and verify**

Run:
```bash
go get github.com/charmbracelet/lipgloss@latest
go mod tidy
go build -o mob .
./mob tui --help
```

Expected: Shows TUI help

**Step 5: Commit**

```bash
git add internal/tui/ cmd/tui.go
git commit -m "feat: implement basic TUI with tabbed interface"
```

---

## Summary

This plan covers the foundational phases to get a working `mob` CLI:

| Phase | Description | Tasks |
|-------|-------------|-------|
| 1 | Project Foundation | Go module, directory structure, version command |
| 2 | Configuration & Data | Config loading, bead storage, turf management |
| 3 | Core CLI Commands | Init wizard, turf commands, add/status commands |
| 4 | Daemon Foundation | PID management, daemon start/stop/status |
| 5 | Soldati Management | Soldati storage, CLI commands, auto-naming |
| 6 | TUI Foundation | Basic TUI shell with tabs |

**Next phases (not detailed here):**
- Phase 7: IPC & Claude Code Integration
- Phase 8: Underboss Chat Interface
- Phase 9: Agent Patrol & Recovery
- Phase 10: Merge Queue & Git Integration
- Phase 11: Sweeps & Heresy Detection

Each task follows TDD (test first), produces working code, and ends with a commit.
