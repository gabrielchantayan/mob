package patrol

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gabe/mob/internal/agent"
)

// mockSpawner is a mock implementation for testing
type mockSpawner struct {
	agents map[string]*agent.Agent
	mu     sync.RWMutex
}

func newMockSpawner() *mockSpawner {
	return &mockSpawner{
		agents: make(map[string]*agent.Agent),
	}
}

func (m *mockSpawner) addAgent(a *agent.Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[a.ID] = a
}

func (m *mockSpawner) List() []*agent.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agents := make([]*agent.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		agents = append(agents, a)
	}
	return agents
}

func (m *mockSpawner) Get(id string) (*agent.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[id]
	return a, ok
}

// mockRunningChecker is a mock implementation that returns configurable running status
type mockRunningChecker struct {
	running map[string]bool
	mu      sync.RWMutex
}

func newMockRunningChecker() *mockRunningChecker {
	return &mockRunningChecker{
		running: make(map[string]bool),
	}
}

func (m *mockRunningChecker) setRunning(agentID string, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running[agentID] = running
}

func (m *mockRunningChecker) IsRunning(agentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running[agentID]
}

func TestPatrol_New(t *testing.T) {
	spawner := newMockSpawner()
	p := New(spawner)

	if p == nil {
		t.Fatal("expected patrol instance, got nil")
	}

	// Verify default interval is 2 minutes
	if p.interval != 2*time.Minute {
		t.Errorf("expected interval 2m, got %v", p.interval)
	}

	// Verify default stuck timeout is 10 minutes
	if p.stuckTimeout != 10*time.Minute {
		t.Errorf("expected stuckTimeout 10m, got %v", p.stuckTimeout)
	}

	// Verify agentStatus map is initialized
	if p.agentStatus == nil {
		t.Error("expected agentStatus map to be initialized")
	}
}

func TestPatrol_WithOptions(t *testing.T) {
	spawner := newMockSpawner()

	var stuckCalled, deadCalled bool
	onStuck := func(status AgentStatus) { stuckCalled = true }
	onDead := func(status AgentStatus) { deadCalled = true }

	p := New(spawner,
		WithInterval(30*time.Second),
		WithStuckTimeout(5*time.Minute),
		WithOnStuck(onStuck),
		WithOnDead(onDead),
	)

	if p.interval != 30*time.Second {
		t.Errorf("expected interval 30s, got %v", p.interval)
	}

	if p.stuckTimeout != 5*time.Minute {
		t.Errorf("expected stuckTimeout 5m, got %v", p.stuckTimeout)
	}

	// Verify callbacks are set by calling them
	if p.onStuck == nil {
		t.Error("expected onStuck callback to be set")
	} else {
		p.onStuck(AgentStatus{})
		if !stuckCalled {
			t.Error("onStuck callback was not properly set")
		}
	}

	if p.onDead == nil {
		t.Error("expected onDead callback to be set")
	} else {
		p.onDead(AgentStatus{})
		if !deadCalled {
			t.Error("onDead callback was not properly set")
		}
	}
}

func TestPatrol_Check(t *testing.T) {
	spawner := newMockSpawner()

	// Add a mock agent
	testAgent := &agent.Agent{
		ID:        "test-agent-1",
		Type:      agent.AgentTypeSoldati,
		Name:      "vinnie",
		Turf:      "/tmp/test",
		StartedAt: time.Now(),
	}
	spawner.addAgent(testAgent)

	p := New(spawner)

	// Check the agent
	status, err := p.Check("test-agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status == nil {
		t.Fatal("expected status, got nil")
	}

	if status.AgentID != "test-agent-1" {
		t.Errorf("expected agent ID test-agent-1, got %s", status.AgentID)
	}

	if status.Name != "vinnie" {
		t.Errorf("expected name vinnie, got %s", status.Name)
	}

	if status.Type != agent.AgentTypeSoldati {
		t.Errorf("expected type soldati, got %s", status.Type)
	}

	// Agent without process should be "dead"
	if status.Status != "dead" {
		t.Errorf("expected status dead for agent without process, got %s", status.Status)
	}
}

func TestPatrol_Check_NotFound(t *testing.T) {
	spawner := newMockSpawner()
	p := New(spawner)

	_, err := p.Check("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}

	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestPatrol_Status(t *testing.T) {
	spawner := newMockSpawner()

	// Add multiple agents
	for i, name := range []string{"agent1", "agent2", "agent3"} {
		spawner.addAgent(&agent.Agent{
			ID:        name,
			Type:      agent.AgentTypeSoldati,
			Name:      name,
			StartedAt: time.Now().Add(time.Duration(-i) * time.Hour),
		})
	}

	p := New(spawner)

	// Do an initial check to populate statuses
	p.checkAll()

	statuses := p.Status()

	if len(statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(statuses))
	}

	// Verify we have all agents
	seen := make(map[string]bool)
	for _, s := range statuses {
		seen[s.AgentID] = true
	}

	for _, name := range []string{"agent1", "agent2", "agent3"} {
		if !seen[name] {
			t.Errorf("missing status for agent %s", name)
		}
	}
}

func TestPatrol_DetectsStuck(t *testing.T) {
	spawner := newMockSpawner()
	runningChecker := newMockRunningChecker()

	// Add an agent that started a while ago
	testAgent := &agent.Agent{
		ID:        "stuck-agent",
		Type:      agent.AgentTypeSoldati,
		Name:      "stuck",
		StartedAt: time.Now().Add(-30 * time.Minute), // Started 30 minutes ago
	}
	spawner.addAgent(testAgent)

	// Mark the agent as running
	runningChecker.setRunning("stuck-agent", true)

	var stuckStatuses []AgentStatus
	var mu sync.Mutex

	p := New(spawner,
		WithStuckTimeout(5*time.Minute), // Stuck after 5 minutes of no activity
		WithRunningChecker(runningChecker),
		WithOnStuck(func(status AgentStatus) {
			mu.Lock()
			defer mu.Unlock()
			stuckStatuses = append(stuckStatuses, status)
		}),
	)

	// Simulate the agent having no recent bead updates
	// by setting its last seen time to be very old
	p.mu.Lock()
	p.agentStatus["stuck-agent"] = &AgentStatus{
		AgentID:        "stuck-agent",
		Name:           "stuck",
		Type:           agent.AgentTypeSoldati,
		Status:         "healthy",
		LastSeen:       time.Now().Add(-15 * time.Minute), // Last seen 15 minutes ago
		LastBeadUpdate: time.Now().Add(-15 * time.Minute), // No bead updates for 15 minutes
	}
	p.mu.Unlock()

	// Check all agents
	p.checkAll()

	mu.Lock()
	defer mu.Unlock()

	if len(stuckStatuses) == 0 {
		t.Error("expected stuck callback to be called")
	}

	if len(stuckStatuses) > 0 && stuckStatuses[0].AgentID != "stuck-agent" {
		t.Errorf("expected stuck-agent, got %s", stuckStatuses[0].AgentID)
	}

	if len(stuckStatuses) > 0 && stuckStatuses[0].Status != "stuck" {
		t.Errorf("expected status stuck, got %s", stuckStatuses[0].Status)
	}
}

func TestPatrol_DetectsDead(t *testing.T) {
	spawner := newMockSpawner()

	// Add an agent without a spawner (dead)
	testAgent := &agent.Agent{
		ID:        "dead-agent",
		Type:      agent.AgentTypeSoldati,
		Name:      "dead",
		StartedAt: time.Now().Add(-10 * time.Minute),
		// No spawner = dead in new architecture
	}
	spawner.addAgent(testAgent)

	var deadStatuses []AgentStatus
	var mu sync.Mutex

	p := New(spawner,
		WithOnDead(func(status AgentStatus) {
			mu.Lock()
			defer mu.Unlock()
			deadStatuses = append(deadStatuses, status)
		}),
	)

	// Check all agents
	p.checkAll()

	mu.Lock()
	defer mu.Unlock()

	if len(deadStatuses) == 0 {
		t.Error("expected dead callback to be called")
	}

	if len(deadStatuses) > 0 && deadStatuses[0].AgentID != "dead-agent" {
		t.Errorf("expected dead-agent, got %s", deadStatuses[0].AgentID)
	}

	if len(deadStatuses) > 0 && deadStatuses[0].Status != "dead" {
		t.Errorf("expected status dead, got %s", deadStatuses[0].Status)
	}
}

func TestPatrol_Start_StopsOnCancel(t *testing.T) {
	spawner := newMockSpawner()

	p := New(spawner, WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- p.Start(ctx)
	}()

	// Let it run a few ticks
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Should stop quickly
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("patrol did not stop after context cancellation")
	}
}

func TestPatrol_UpdateLastSeen(t *testing.T) {
	spawner := newMockSpawner()

	testAgent := &agent.Agent{
		ID:        "test-agent",
		Type:      agent.AgentTypeSoldati,
		Name:      "test",
		StartedAt: time.Now(),
	}
	spawner.addAgent(testAgent)

	p := New(spawner)

	// First check
	before := time.Now()
	p.checkAll()
	after := time.Now()

	p.mu.RLock()
	status := p.agentStatus["test-agent"]
	p.mu.RUnlock()

	if status == nil {
		t.Fatal("expected status to be recorded")
	}

	if status.LastSeen.Before(before) || status.LastSeen.After(after) {
		t.Errorf("LastSeen %v not in expected range [%v, %v]", status.LastSeen, before, after)
	}
}

func TestPatrol_UpdateBeadTime(t *testing.T) {
	spawner := newMockSpawner()

	testAgent := &agent.Agent{
		ID:        "test-agent",
		Type:      agent.AgentTypeSoldati,
		Name:      "test",
		StartedAt: time.Now(),
	}
	spawner.addAgent(testAgent)

	p := New(spawner)

	// Update bead time
	beadTime := time.Now()
	p.UpdateBeadTime("test-agent", beadTime)

	p.mu.RLock()
	status := p.agentStatus["test-agent"]
	p.mu.RUnlock()

	if status == nil {
		t.Fatal("expected status to be created")
	}

	if !status.LastBeadUpdate.Equal(beadTime) {
		t.Errorf("expected LastBeadUpdate %v, got %v", beadTime, status.LastBeadUpdate)
	}
}
