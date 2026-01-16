package agent

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"github.com/gabe/mob/internal/ipc"
)

func TestSpawner_SpawnGeneratesID(t *testing.T) {
	spawner := NewSpawner()

	// Use a mock command creator that creates a simple process
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		// Use "true" command which exits immediately (available on Unix)
		return exec.Command("true")
	})

	// Spawn multiple agents and verify unique IDs
	agent1, err := spawner.Spawn(AgentTypeSoldati, "vinnie", "project-a", "/tmp")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	defer spawner.KillAll()

	agent2, err := spawner.Spawn(AgentTypeSoldati, "sal", "project-b", "/tmp")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Verify IDs are not empty
	if agent1.ID == "" {
		t.Error("agent1.ID should not be empty")
	}
	if agent2.ID == "" {
		t.Error("agent2.ID should not be empty")
	}

	// Verify IDs are unique
	if agent1.ID == agent2.ID {
		t.Errorf("agent IDs should be unique, got %s for both", agent1.ID)
	}

	// Verify ID length (16 hex chars from 8 bytes)
	if len(agent1.ID) != 16 {
		t.Errorf("expected ID length 16, got %d", len(agent1.ID))
	}
}

func TestAgent_IsRunning(t *testing.T) {
	// Test with nil Cmd
	agent := &Agent{}
	if agent.IsRunning() {
		t.Error("agent with nil Cmd should not be running")
	}

	// Test with Cmd but nil Process
	agent.Cmd = &exec.Cmd{}
	if agent.IsRunning() {
		t.Error("agent with nil Process should not be running")
	}

	// Test with actual running process
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	defer cmd.Process.Kill()

	agent.Cmd = cmd
	if !agent.IsRunning() {
		t.Error("agent with running process should be running")
	}

	// Kill and verify not running
	cmd.Process.Kill()
	cmd.Wait() // Wait for process to fully terminate

	if agent.IsRunning() {
		t.Error("agent should not be running after kill")
	}
}

func TestSpawner_List(t *testing.T) {
	spawner := NewSpawner()

	// Use a mock command creator
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	})

	// Initially empty
	agents := spawner.List()
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}

	// Spawn some agents
	names := []string{"vinnie", "sal", "tony"}
	for _, name := range names {
		_, err := spawner.Spawn(AgentTypeSoldati, name, "turf", "/tmp")
		if err != nil {
			t.Fatalf("Spawn failed: %v", err)
		}
	}
	defer spawner.KillAll()

	// List should return all agents
	agents = spawner.List()
	if len(agents) != len(names) {
		t.Errorf("expected %d agents, got %d", len(names), len(agents))
	}

	// Verify all names are present
	foundNames := make(map[string]bool)
	for _, agent := range agents {
		foundNames[agent.Name] = true
	}
	for _, name := range names {
		if !foundNames[name] {
			t.Errorf("expected to find agent %q in list", name)
		}
	}
}

func TestSpawner_Get(t *testing.T) {
	spawner := NewSpawner()

	// Use a mock command creator
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	})

	// Get non-existent agent
	_, ok := spawner.Get("nonexistent")
	if ok {
		t.Error("expected Get to return false for non-existent agent")
	}

	// Spawn an agent
	agent, err := spawner.Spawn(AgentTypeSoldati, "vinnie", "turf", "/tmp")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	defer spawner.KillAll()

	// Get the agent by ID
	retrieved, ok := spawner.Get(agent.ID)
	if !ok {
		t.Error("expected Get to return true for existing agent")
	}
	if retrieved.ID != agent.ID {
		t.Errorf("expected ID %s, got %s", agent.ID, retrieved.ID)
	}
	if retrieved.Name != "vinnie" {
		t.Errorf("expected name vinnie, got %s", retrieved.Name)
	}
	if retrieved.Turf != "turf" {
		t.Errorf("expected turf 'turf', got %s", retrieved.Turf)
	}
	if retrieved.Type != AgentTypeSoldati {
		t.Errorf("expected type %s, got %s", AgentTypeSoldati, retrieved.Type)
	}
}

func TestSpawner_Kill(t *testing.T) {
	spawner := NewSpawner()

	// Use a long-running process that we can kill
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return exec.Command("sleep", "60")
	})

	// Kill non-existent agent
	err := spawner.Kill("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}

	// Spawn an agent
	agent, err := spawner.Spawn(AgentTypeSoldati, "vinnie", "turf", "/tmp")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Verify it's tracked
	if spawner.Count() != 1 {
		t.Errorf("expected 1 agent, got %d", spawner.Count())
	}

	// Kill it
	err = spawner.Kill(agent.ID)
	if err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	// Verify it's removed
	if spawner.Count() != 0 {
		t.Errorf("expected 0 agents after kill, got %d", spawner.Count())
	}

	// Get should fail now
	_, ok := spawner.Get(agent.ID)
	if ok {
		t.Error("expected Get to return false after kill")
	}
}

func TestSpawner_KillAll(t *testing.T) {
	spawner := NewSpawner()

	// Use a long-running process
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return exec.Command("sleep", "60")
	})

	// Spawn multiple agents
	for i := 0; i < 3; i++ {
		_, err := spawner.Spawn(AgentTypeSoldati, "agent", "turf", "/tmp")
		if err != nil {
			t.Fatalf("Spawn failed: %v", err)
		}
	}

	// Verify they're tracked
	if spawner.Count() != 3 {
		t.Errorf("expected 3 agents, got %d", spawner.Count())
	}

	// Kill all
	spawner.KillAll()

	// Verify all are removed
	if spawner.Count() != 0 {
		t.Errorf("expected 0 agents after KillAll, got %d", spawner.Count())
	}
}

func TestNewSpawnerWithPath(t *testing.T) {
	spawner := NewSpawnerWithPath("/custom/path/to/claude")

	if spawner.claudePath != "/custom/path/to/claude" {
		t.Errorf("expected claudePath '/custom/path/to/claude', got %s", spawner.claudePath)
	}
	if spawner.agents == nil {
		t.Error("agents map should be initialized")
	}
	if spawner.commandCreator == nil {
		t.Error("commandCreator should be initialized")
	}
}

func TestAgent_Send(t *testing.T) {
	// Test with nil client
	agent := &Agent{}
	err := agent.Send("test/method", nil)
	if err != ErrAgentNotConnected {
		t.Errorf("expected ErrAgentNotConnected, got %v", err)
	}

	// Test with connected client
	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	agent.Client = ipc.NewClient(stdin, stdout)

	err = agent.Send("test/method", map[string]string{"key": "value"})
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Verify something was written to stdin
	if stdin.Len() == 0 {
		t.Error("expected data to be written to stdin")
	}
}

func TestAgent_Call(t *testing.T) {
	// Test with nil client
	agent := &Agent{}
	_, err := agent.Call("test/method", nil)
	if err != ErrAgentNotConnected {
		t.Errorf("expected ErrAgentNotConnected, got %v", err)
	}
}

func TestAgent_Wait(t *testing.T) {
	// Test with nil Cmd
	agent := &Agent{}
	err := agent.Wait()
	if err != ErrAgentNotStarted {
		t.Errorf("expected ErrAgentNotStarted, got %v", err)
	}

	// Test with actual command
	cmd := exec.Command("true")
	cmd.Start()
	agent.Cmd = cmd

	err = agent.Wait()
	if err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

func TestAgent_Kill(t *testing.T) {
	// Test with nil Cmd
	agent := &Agent{}
	err := agent.Kill()
	if err != ErrAgentNotStarted {
		t.Errorf("expected ErrAgentNotStarted, got %v", err)
	}

	// Test with Cmd but nil Process
	agent.Cmd = &exec.Cmd{}
	err = agent.Kill()
	if err != ErrAgentNotStarted {
		t.Errorf("expected ErrAgentNotStarted, got %v", err)
	}

	// Test with actual running process
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	agent.Cmd = cmd

	err = agent.Kill()
	if err != nil {
		t.Errorf("Kill failed: %v", err)
	}
}

func TestAgent_Properties(t *testing.T) {
	now := time.Now()
	agent := &Agent{
		ID:        "test-id-123",
		Type:      AgentTypeUnderboss,
		Name:      "don",
		Turf:      "main-project",
		StartedAt: now,
	}

	if agent.ID != "test-id-123" {
		t.Errorf("expected ID 'test-id-123', got %s", agent.ID)
	}
	if agent.Type != AgentTypeUnderboss {
		t.Errorf("expected Type %s, got %s", AgentTypeUnderboss, agent.Type)
	}
	if agent.Name != "don" {
		t.Errorf("expected Name 'don', got %s", agent.Name)
	}
	if agent.Turf != "main-project" {
		t.Errorf("expected Turf 'main-project', got %s", agent.Turf)
	}
	if !agent.StartedAt.Equal(now) {
		t.Errorf("expected StartedAt %v, got %v", now, agent.StartedAt)
	}
}

func TestAgentTypes(t *testing.T) {
	// Verify agent type constants
	if AgentTypeUnderboss != "underboss" {
		t.Errorf("expected AgentTypeUnderboss='underboss', got %s", AgentTypeUnderboss)
	}
	if AgentTypeSoldati != "soldati" {
		t.Errorf("expected AgentTypeSoldati='soldati', got %s", AgentTypeSoldati)
	}
	if AgentTypeAssociate != "associate" {
		t.Errorf("expected AgentTypeAssociate='associate', got %s", AgentTypeAssociate)
	}
}

func TestSpawner_Count(t *testing.T) {
	spawner := NewSpawner()

	// Use a mock command creator
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	})

	// Initially zero
	if spawner.Count() != 0 {
		t.Errorf("expected count 0, got %d", spawner.Count())
	}

	// Spawn some agents
	for i := 0; i < 5; i++ {
		_, err := spawner.Spawn(AgentTypeSoldati, "agent", "turf", "/tmp")
		if err != nil {
			t.Fatalf("Spawn failed: %v", err)
		}
	}
	defer spawner.KillAll()

	// Count should reflect spawned agents
	if spawner.Count() != 5 {
		t.Errorf("expected count 5, got %d", spawner.Count())
	}
}

func TestGenerateID(t *testing.T) {
	// Generate multiple IDs and verify uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true

		// Verify length
		if len(id) != 16 {
			t.Errorf("expected ID length 16, got %d", len(id))
		}
	}
}
