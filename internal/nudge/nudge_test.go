package nudge

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/hook"
)

// mockAgent is a test helper to create a minimal agent with stdin control
type mockAgent struct {
	*agent.Agent
	stdinBuf *mockWriter
}

// mockWriter captures writes for testing
type mockWriter struct {
	mu   sync.Mutex
	data []byte
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *mockWriter) GetData() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.data...)
}

func TestNudger_NudgeStdin(t *testing.T) {
	// Create a mock spawner with a test agent
	spawner := agent.NewSpawner()

	// Create temp directory for hooks
	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	// Create a mock writer to capture stdin
	mockStdin := &mockWriter{}

	// We need to add a test agent to the spawner
	// Since spawner.Spawn actually starts a process, we'll use a different approach
	// We'll test the nudge logic by mocking at the Nudger level
	testAgent := &agent.Agent{
		ID:   "test-agent-1",
		Type: agent.AgentTypeSoldati,
		Name: "vinnie",
	}

	// Register the agent with our nudger's internal tracking
	nudger.RegisterAgent(testAgent, mockStdin)

	// Perform level 0 nudge (stdin)
	err = nudger.Nudge("test-agent-1", LevelStdin)
	if err != nil {
		t.Errorf("Nudge(LevelStdin) returned error: %v", err)
	}

	// Verify newline was written to stdin
	data := mockStdin.GetData()
	if string(data) != "\n" {
		t.Errorf("expected newline written to stdin, got: %q", string(data))
	}

	// Check that the event was recorded in history
	history := nudger.History("test-agent-1")
	if len(history) != 1 {
		t.Fatalf("expected 1 history event, got %d", len(history))
	}
	if history[0].Level != LevelStdin {
		t.Errorf("expected level LevelStdin, got %v", history[0].Level)
	}
	if !history[0].Success {
		t.Errorf("expected success=true, got false")
	}
}

func TestNudger_NudgeHook(t *testing.T) {
	spawner := agent.NewSpawner()

	// Create temp directory for hooks
	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	// Create a test agent
	testAgent := &agent.Agent{
		ID:   "test-agent-2",
		Type: agent.AgentTypeSoldati,
		Name: "tony",
	}

	mockStdin := &mockWriter{}
	nudger.RegisterAgent(testAgent, mockStdin)

	// Perform level 1 nudge (hook file)
	err = nudger.Nudge("test-agent-2", LevelHook)
	if err != nil {
		t.Errorf("Nudge(LevelHook) returned error: %v", err)
	}

	// Verify hook file was created with nudge type
	hookPath := filepath.Join(tmpDir, "tony", "hook.json")
	hookMgr, err := hook.NewManager(tmpDir, "tony")
	if err != nil {
		t.Fatalf("failed to create hook manager: %v", err)
	}

	hookData, err := hookMgr.Read()
	if err != nil {
		t.Fatalf("failed to read hook file: %v", err)
	}
	if hookData == nil {
		t.Fatalf("hook file not found at %s", hookPath)
	}
	if hookData.Type != hook.HookTypeNudge {
		t.Errorf("expected hook type 'nudge', got %q", hookData.Type)
	}

	// Check history
	history := nudger.History("test-agent-2")
	if len(history) != 1 {
		t.Fatalf("expected 1 history event, got %d", len(history))
	}
	if history[0].Level != LevelHook {
		t.Errorf("expected level LevelHook, got %v", history[0].Level)
	}
}

func TestNudger_History(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	testAgent := &agent.Agent{
		ID:   "test-agent-3",
		Type: agent.AgentTypeSoldati,
		Name: "sal",
	}

	mockStdin := &mockWriter{}
	nudger.RegisterAgent(testAgent, mockStdin)

	// Perform multiple nudges
	nudger.Nudge("test-agent-3", LevelStdin)
	nudger.Nudge("test-agent-3", LevelStdin)
	nudger.Nudge("test-agent-3", LevelHook)

	// Check history has all events
	history := nudger.History("test-agent-3")
	if len(history) != 3 {
		t.Fatalf("expected 3 history events, got %d", len(history))
	}

	// Verify order and levels
	if history[0].Level != LevelStdin {
		t.Errorf("history[0]: expected LevelStdin, got %v", history[0].Level)
	}
	if history[1].Level != LevelStdin {
		t.Errorf("history[1]: expected LevelStdin, got %v", history[1].Level)
	}
	if history[2].Level != LevelHook {
		t.Errorf("history[2]: expected LevelHook, got %v", history[2].Level)
	}

	// Verify times are ordered
	if !history[1].Time.After(history[0].Time) && !history[1].Time.Equal(history[0].Time) {
		t.Errorf("history events not in chronological order")
	}
}

func TestNudger_NudgeNotFound(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	// Try to nudge a non-existent agent
	err = nudger.Nudge("nonexistent-agent", LevelStdin)
	if err == nil {
		t.Error("expected error for non-existent agent, got nil")
	}
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

func TestNudger_NudgeEscalating(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nudger with shorter delays for testing
	nudger := New(spawner, tmpDir)
	nudger.SetEscalationDelay(10 * time.Millisecond) // Short delay for tests

	testAgent := &agent.Agent{
		ID:   "test-agent-4",
		Type: agent.AgentTypeSoldati,
		Name: "frankie",
	}

	mockStdin := &mockWriter{}
	nudger.RegisterAgent(testAgent, mockStdin)

	// Test escalating nudge with a context that times out
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should attempt level 0 and succeed, returning immediately
	err = nudger.NudgeEscalating(ctx, "test-agent-4")
	if err != nil {
		t.Errorf("NudgeEscalating returned error: %v", err)
	}

	// Since level 0 succeeds, only 1 attempt should be recorded
	history := nudger.History("test-agent-4")
	if len(history) != 1 {
		t.Errorf("expected 1 escalation attempt (success at level 0), got %d", len(history))
	}
	if history[0].Level != LevelStdin {
		t.Errorf("expected level LevelStdin, got %v", history[0].Level)
	}
	if !history[0].Success {
		t.Errorf("expected success=true for level 0")
	}
}

func TestNudger_NudgeEscalating_AllLevels(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nudger with shorter delays for testing
	nudger := New(spawner, tmpDir)
	nudger.SetEscalationDelay(10 * time.Millisecond) // Short delay for tests

	testAgent := &agent.Agent{
		ID:   "test-agent-escalate",
		Type: agent.AgentTypeSoldati,
		Name: "escalate-test",
	}

	// Register with nil stdin to make level 0 fail
	nudger.RegisterAgent(testAgent, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Level 0 will fail (nil stdin), level 1 should succeed (hook file)
	err = nudger.NudgeEscalating(ctx, "test-agent-escalate")
	if err != nil {
		t.Errorf("NudgeEscalating returned error: %v", err)
	}

	// Should have attempted level 0 (fail) then level 1 (success)
	history := nudger.History("test-agent-escalate")
	if len(history) != 2 {
		t.Errorf("expected 2 escalation attempts, got %d", len(history))
	}
	if len(history) >= 2 {
		if history[0].Level != LevelStdin || history[0].Success {
			t.Errorf("expected level 0 to fail, got level=%v success=%v", history[0].Level, history[0].Success)
		}
		if history[1].Level != LevelHook || !history[1].Success {
			t.Errorf("expected level 1 to succeed, got level=%v success=%v", history[1].Level, history[1].Success)
		}
	}
}

func TestNudgeLevel_String(t *testing.T) {
	tests := []struct {
		level    NudgeLevel
		expected string
	}{
		{LevelStdin, "stdin"},
		{LevelHook, "hook"},
		{LevelRestart, "restart"},
		{NudgeLevel(99), "unknown(99)"},
	}

	for _, tt := range tests {
		result := tt.level.String()
		if result != tt.expected {
			t.Errorf("NudgeLevel(%d).String() = %q, want %q", tt.level, result, tt.expected)
		}
	}
}

func TestNudger_GetByName(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	testAgent := &agent.Agent{
		ID:   "test-agent-5",
		Type: agent.AgentTypeSoldati,
		Name: "paulie",
	}

	mockStdin := &mockWriter{}
	nudger.RegisterAgent(testAgent, mockStdin)

	// Test getting agent by name
	agent, ok := nudger.GetByName("paulie")
	if !ok {
		t.Error("expected to find agent 'paulie'")
	}
	if agent.ID != "test-agent-5" {
		t.Errorf("expected agent ID 'test-agent-5', got %q", agent.ID)
	}

	// Test getting non-existent name
	_, ok = nudger.GetByName("nonexistent")
	if ok {
		t.Error("expected not to find agent 'nonexistent'")
	}
}

func TestNudger_NudgeByName(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	testAgent := &agent.Agent{
		ID:   "test-agent-6",
		Type: agent.AgentTypeSoldati,
		Name: "joey",
	}

	mockStdin := &mockWriter{}
	nudger.RegisterAgent(testAgent, mockStdin)

	// Test nudging by name
	err = nudger.NudgeByName("joey", LevelStdin)
	if err != nil {
		t.Errorf("NudgeByName returned error: %v", err)
	}

	// Verify the nudge was recorded
	history := nudger.History("test-agent-6")
	if len(history) != 1 {
		t.Fatalf("expected 1 history event, got %d", len(history))
	}

	// Test nudging non-existent name
	err = nudger.NudgeByName("nonexistent", LevelStdin)
	if err == nil {
		t.Error("expected error for non-existent name, got nil")
	}
}

func TestNudger_ListAgents(t *testing.T) {
	spawner := agent.NewSpawner()

	tmpDir, err := os.MkdirTemp("", "nudge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nudger := New(spawner, tmpDir)

	// Register multiple agents
	agents := []*agent.Agent{
		{ID: "agent-1", Type: agent.AgentTypeSoldati, Name: "vinnie"},
		{ID: "agent-2", Type: agent.AgentTypeSoldati, Name: "tony"},
		{ID: "agent-3", Type: agent.AgentTypeSoldati, Name: "sal"},
	}

	for _, a := range agents {
		nudger.RegisterAgent(a, &mockWriter{})
	}

	// List all agents
	list := nudger.ListAgents()
	if len(list) != 3 {
		t.Errorf("expected 3 agents, got %d", len(list))
	}
}
