package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/config"
	"github.com/gabe/mob/internal/hook"
	"github.com/gabe/mob/internal/mcp"
	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/soldati"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/turf"
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
	pidFile      string
	stateFile    string
	mobDir       string
	logger       *log.Logger
	state        State
	ctx          context.Context
	cancel       context.CancelFunc
	spawner      *agent.Spawner
	registry     *registry.Registry
	soldatiMgr   *soldati.Manager
	turfMgr      *turf.Manager
	beadStore    *storage.BeadStore
	activeAgents map[string]*agent.Agent       // keyed by soldati name
	hookManagers map[string]*hook.Manager      // keyed by soldati name
	hookCancels  map[string]context.CancelFunc // keyed by soldati name
	nudgedAt     map[string]time.Time          // keyed by associate ID, tracks when nudge was sent
	mu           sync.RWMutex                  // protects activeAgents, hookManagers, hookCancels, nudgedAt
}

// New creates a new daemon instance
func New(mobDir string, logger *log.Logger) *Daemon {
	return &Daemon{
		pidFile:      filepath.Join(mobDir, ".mob", "daemon.pid"),
		stateFile:    filepath.Join(mobDir, ".mob", "daemon.state"),
		mobDir:       mobDir,
		logger:       logger,
		state:        StateIdle,
		activeAgents: make(map[string]*agent.Agent),
		hookManagers: make(map[string]*hook.Manager),
		hookCancels:  make(map[string]context.CancelFunc),
		nudgedAt:     make(map[string]time.Time),
	}
}

// Start begins daemon operation
func (d *Daemon) Start() error {
	// Create .mob directory if it doesn't exist
	mobDir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(mobDir, 0755); err != nil {
		return fmt.Errorf("failed to create .mob directory: %w", err)
	}

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

	// Initialize spawner, registry, soldati manager, and turf manager
	d.spawner = agent.NewSpawner()
	d.registry = registry.New(registry.DefaultPath(d.mobDir))
	soldatiDir := filepath.Join(d.mobDir, "soldati")
	if err := os.MkdirAll(soldatiDir, 0755); err != nil {
		return fmt.Errorf("failed to create soldati directory: %w", err)
	}
	soldatiMgr, err := soldati.NewManager(soldatiDir)
	if err != nil {
		return fmt.Errorf("failed to create soldati manager: %w", err)
	}
	d.soldatiMgr = soldatiMgr

	// Initialize turf manager for resolving turf names to paths
	turfsPath := filepath.Join(d.mobDir, "turfs.toml")
	turfMgr, err := turf.NewManager(turfsPath)
	if err != nil {
		return fmt.Errorf("failed to create turf manager: %w", err)
	}
	d.turfMgr = turfMgr

	// Initialize bead store for auto-assignment
	beadsDir := filepath.Join(d.mobDir, "beads")
	beadStore, err := storage.NewBeadStore(beadsDir)
	if err != nil {
		return fmt.Errorf("failed to create bead store: %w", err)
	}
	d.beadStore = beadStore

	// Set up context for graceful shutdown
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.state = StateRunning

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	d.logger.Println("Mob daemon started")

	// Run initial patrol immediately
	d.patrol()

	// Main loop with two tickers:
	// - patrol every 2 minutes (health checks, spawning, cleanup)
	// - nudge all agents every 5 minutes (keep them working)
	patrolTicker := time.NewTicker(2 * time.Minute)
	nudgeTicker := time.NewTicker(5 * time.Minute)
	defer patrolTicker.Stop()
	defer nudgeTicker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return d.shutdown()
		case sig := <-sigChan:
			d.logger.Printf("\nReceived signal %v, shutting down...\n", sig)
			return d.shutdown()
		case <-patrolTicker.C:
			d.patrol()
		case <-nudgeTicker.C:
			d.nudgeAllAgents()
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

	d.mu.Lock()
	// Cancel all hook watchers
	for name, cancel := range d.hookCancels {
		d.logger.Printf("Stopping hook watcher for '%s'\n", name)
		cancel()
	}
	d.hookCancels = make(map[string]context.CancelFunc)
	d.hookManagers = make(map[string]*hook.Manager)

	// Kill all active agents
	for name, a := range d.activeAgents {
		d.logger.Printf("Stopping soldati '%s'\n", name)
		a.Kill()
	}
	d.activeAgents = make(map[string]*agent.Agent)
	d.mu.Unlock()

	// Clear registry entries for our agents
	if d.registry != nil {
		agents, _ := d.registry.ListByType("soldati")
		for _, a := range agents {
			d.registry.Unregister(a.ID)
		}
	}

	RemovePID(d.pidFile)
	d.logger.Println("Mob daemon stopped")
	return nil
}

func (d *Daemon) patrol() {
	if d.soldatiMgr == nil || d.spawner == nil || d.registry == nil {
		return
	}

	// Check associate timeouts and clean up stale ones
	d.patrolAssociates()
	d.cleanupStaleAssociates()

	// Get all registered soldati from TOML files
	registeredSoldati, err := d.soldatiMgr.List()
	if err != nil {
		d.logger.Printf("Patrol: failed to list soldati: %v\n", err)
		return
	}

	if len(registeredSoldati) == 0 {
		return
	}

	// Get all active soldati from registry
	activeAgents, err := d.registry.ListByType("soldati")
	if err != nil {
		d.logger.Printf("Patrol: failed to list active agents: %v\n", err)
		return
	}

	// Build map of active agent names
	activeNames := make(map[string]*registry.AgentRecord)
	for _, a := range activeAgents {
		activeNames[a.Name] = a
	}

	// Spawn Claude instances for soldati that don't have active agents
	for _, s := range registeredSoldati {
		if _, active := activeNames[s.Name]; active {
			// Already has an active agent, check health
			d.checkAgentHealth(s.Name, activeNames[s.Name])
			continue
		}

		// Check if we already have this agent in memory
		if existingAgent, ok := d.activeAgents[s.Name]; ok && existingAgent.IsRunning() {
			continue
		}

		// Spawn a new Claude instance for this soldati
		d.logger.Printf("Patrol: spawning Claude instance for soldati '%s'\n", s.Name)
		if err := d.spawnSoldatiAgent(s.Name); err != nil {
			d.logger.Printf("Patrol: failed to spawn agent for '%s': %v\n", s.Name, err)
		}
	}

	// Clean up stale registry entries for soldati that no longer exist
	for name, record := range activeNames {
		found := false
		for _, s := range registeredSoldati {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			d.logger.Printf("Patrol: removing stale registry entry for '%s'\n", name)
			d.registry.Unregister(record.ID)
			delete(d.activeAgents, name)
		}
	}

	// Auto-assign work to idle agents
	d.assignWorkToIdleAgents()
}

// assignWorkToIdleAgents checks for idle soldati and assigns them the next ready bead
func (d *Daemon) assignWorkToIdleAgents() {
	if d.beadStore == nil {
		return
	}

	// Get all active soldati from registry
	agents, err := d.registry.ListByType("soldati")
	if err != nil {
		d.logger.Printf("Patrol: failed to list agents for auto-assign: %v\n", err)
		return
	}

	for _, agentRecord := range agents {
		// Only assign to idle agents
		if agentRecord.Status != "idle" {
			continue
		}

		// Check if agent has an empty hook (no pending work)
		d.mu.RLock()
		hookMgr, hasHook := d.hookManagers[agentRecord.Name]
		d.mu.RUnlock()

		if hasHook {
			hook, _ := hookMgr.Read()
			if hook != nil {
				// Hook has work, skip
				continue
			}
		}

		// Find next ready bead for this agent's turf
		readyBeads, err := d.beadStore.ListReady(agentRecord.Turf)
		if err != nil || len(readyBeads) == 0 {
			continue
		}

		// Pick first (highest priority) ready bead
		nextBead := readyBeads[0]

		d.logger.Printf("Patrol: auto-assigning bead %s to idle agent '%s'\n",
			nextBead.ID, agentRecord.Name)

		// Assign via hook (same as assign_bead MCP tool)
		if err := d.AssignWork(agentRecord.Name, nextBead.ID, nextBead.Title); err != nil {
			d.logger.Printf("Patrol: failed to auto-assign: %v\n", err)
			continue
		}

		// Update bead status and assignee
		nextBead.Status = models.BeadStatusInProgress
		nextBead.Assignee = agentRecord.Name
		if _, err := d.beadStore.Update(nextBead); err != nil {
			d.logger.Printf("Patrol: failed to update bead status: %v\n", err)
		}

		// Nudge the agent to check their hook
		d.nudgeAgent(agentRecord.Name)
	}
}

// nudgeAgent sends a nudge to a specific agent to check their hook
func (d *Daemon) nudgeAgent(name string) {
	d.mu.RLock()
	a, ok := d.activeAgents[name]
	d.mu.RUnlock()

	if !ok || !a.IsRunning() {
		return
	}

	go func() {
		d.logger.Printf("Patrol: nudging agent '%s' to check hook\n", name)
		_, err := a.Chat("Check your hook. If there's work, do it.")
		if err != nil {
			d.logger.Printf("Patrol: failed to nudge agent '%s': %v\n", name, err)
		}
	}()
}

// nudgeAllAgents sends a nudge to agents that have tasks assigned.
// This is called every 5 minutes to prevent agents from getting stuck.
// Only nudges agents that have work (hook with assignment or non-idle status).
func (d *Daemon) nudgeAllAgents() {
	// First, try to assign work to any idle agents
	d.assignWorkToIdleAgents()

	d.mu.RLock()
	agents := make(map[string]*agent.Agent)
	hookMgrs := make(map[string]*hook.Manager)
	for name, a := range d.activeAgents {
		agents[name] = a
		if mgr, ok := d.hookManagers[name]; ok {
			hookMgrs[name] = mgr
		}
	}
	d.mu.RUnlock()

	if len(agents) == 0 {
		return
	}

	// Get agent statuses from registry
	agentRecords, err := d.registry.ListByType("soldati")
	if err != nil {
		d.logger.Printf("Nudge: failed to list agents: %v\n", err)
		return
	}

	statusMap := make(map[string]string)
	for _, rec := range agentRecords {
		statusMap[rec.Name] = rec.Status
	}

	nudgeCount := 0
	for name, a := range agents {
		if !a.IsRunning() {
			continue
		}

		// Check if agent has work: either has a hook or is not idle
		hasWork := false

		// Check hook
		if mgr, ok := hookMgrs[name]; ok {
			if h, _ := mgr.Read(); h != nil {
				hasWork = true
			}
		}

		// Check status - if active/working, they have work
		if status, ok := statusMap[name]; ok && status != "idle" {
			hasWork = true
		}

		if !hasWork {
			continue
		}

		nudgeCount++
		// Send a message to the agent via Chat() - this uses --resume to continue the session
		go func(name string, a *agent.Agent) {
			d.logger.Printf("Nudge: nudging soldati '%s'\n", name)
			_, err := a.Chat("Do your job.")
			if err != nil {
				d.logger.Printf("Nudge: failed to nudge soldati '%s': %v\n", name, err)
			}
		}(name, a)
	}

	if nudgeCount > 0 {
		d.logger.Printf("Nudge: sent nudge to %d agents with active work\n", nudgeCount)
	}
}

// patrolAssociates checks all associates for timeouts and handles them.
// Associates that exceed the timeout are first nudged, then force-killed after a grace period.
func (d *Daemon) patrolAssociates() {
	if d.registry == nil || d.spawner == nil {
		return
	}

	// Get all associates from registry
	associates, err := d.registry.ListByType("associate")
	if err != nil {
		d.logger.Printf("Patrol: failed to list associates: %v\n", err)
		return
	}

	if len(associates) == 0 {
		return
	}

	now := time.Now()
	timeout := config.DefaultAssociateTimeout
	gracePeriod := config.DefaultAssociateGracePeriod

	for _, assoc := range associates {
		// Skip completed or failed associates (they should be cleaned up by cleanupStaleAssociates)
		if assoc.Status == "completed" || assoc.Status == "failed" || assoc.Status == "timed_out" {
			continue
		}

		// Calculate how long the associate has been running
		runningTime := now.Sub(assoc.StartedAt)

		// Check if already nudged
		d.mu.RLock()
		nudgeTime, wasNudged := d.nudgedAt[assoc.ID]
		d.mu.RUnlock()

		if wasNudged {
			// Check if grace period has expired
			timeSinceNudge := now.Sub(nudgeTime)
			if timeSinceNudge > gracePeriod {
				// Grace period expired - force kill
				d.forceKillAssociate(assoc, "timeout after nudge grace period")
			}
			// Still in grace period, skip
			continue
		}

		// Check if timeout exceeded
		if runningTime > timeout {
			// First nudge
			d.nudgeAssociate(assoc)
		}
	}
}

// nudgeAssociate sends a nudge signal to a timed-out associate and records the nudge time
func (d *Daemon) nudgeAssociate(assoc *registry.AgentRecord) {
	d.logger.Printf("Patrol: associate '%s' exceeded timeout (running since %s), sending nudge\n",
		assoc.ID, assoc.StartedAt.Format(time.RFC3339))

	// Record nudge time
	d.mu.Lock()
	d.nudgedAt[assoc.ID] = time.Now()
	d.mu.Unlock()

	// Update status to indicate nudged
	d.registry.UpdateStatus(assoc.ID, "nudged")

	// The actual nudge - update the ping time which should trigger activity check
	d.registry.Ping(assoc.ID)

	d.logger.Printf("Patrol: nudged associate '%s', will force kill in %v if no response\n",
		assoc.ID, config.DefaultAssociateGracePeriod)
}

// forceKillAssociate terminates an associate that has exceeded its timeout and grace period
func (d *Daemon) forceKillAssociate(assoc *registry.AgentRecord, reason string) {
	d.logger.Printf("Patrol: force killing associate '%s' - %s\n", assoc.ID, reason)

	// Kill in spawner (if it has a process)
	if err := d.spawner.Kill(assoc.ID); err != nil {
		// Ignore errors - process might already be dead
		d.logger.Printf("Patrol: warning - spawner.Kill failed for '%s': %v\n", assoc.ID, err)
	}

	// Update registry status to timed_out
	d.registry.UpdateStatus(assoc.ID, "timed_out")

	// Clean up nudge tracking
	d.mu.Lock()
	delete(d.nudgedAt, assoc.ID)
	d.mu.Unlock()

	d.logger.Printf("Patrol: associate '%s' terminated due to timeout\n", assoc.ID)
}

// AssociateCleanupTTL is how long after completion before an associate is removed from registry
const AssociateCleanupTTL = 5 * time.Minute

// cleanupStaleAssociates removes associates that have been in a terminal state for too long.
// This catches cases where the self-cleanup in handleSpawnAssociate failed.
func (d *Daemon) cleanupStaleAssociates() {
	if d.registry == nil {
		return
	}

	// Get all associates from registry
	associates, err := d.registry.ListByType("associate")
	if err != nil {
		d.logger.Printf("Patrol: failed to list associates for cleanup: %v\n", err)
		return
	}

	now := time.Now()

	for _, assoc := range associates {
		// Only clean up terminal states
		if assoc.Status != "completed" && assoc.Status != "failed" && assoc.Status != "timed_out" {
			continue
		}

		// Check if cleanup TTL has expired
		var completedTime time.Time
		if assoc.CompletedAt != nil {
			completedTime = *assoc.CompletedAt
		} else {
			// Fallback to LastPing if CompletedAt not set (shouldn't happen but be safe)
			completedTime = assoc.LastPing
		}

		timeSinceCompletion := now.Sub(completedTime)
		if timeSinceCompletion > AssociateCleanupTTL {
			d.logger.Printf("Patrol: cleaning up stale associate '%s' (completed %v ago)\n",
				assoc.ID, timeSinceCompletion.Round(time.Second))

			if err := d.registry.Unregister(assoc.ID); err != nil {
				d.logger.Printf("Patrol: failed to unregister stale associate '%s': %v\n", assoc.ID, err)
			}
		}
	}
}

// spawnSoldatiAgent creates a Claude instance for a soldati
func (d *Daemon) spawnSoldatiAgent(name string) error {
	// Use current working directory as default work dir
	workDir, err := os.Getwd()
	if err != nil {
		workDir = d.mobDir
	}

	// Generate MCP config for tool access
	mcpConfigPath, err := mcp.GenerateMCPConfig(d.mobDir)
	if err != nil {
		d.logger.Printf("Warning: failed to generate MCP config: %v", err)
	}

	// Spawn the agent with system prompt
	a, err := d.spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeSoldati,
		Name:         name,
		Turf:         "", // Will be assigned when work is given
		WorkDir:      workDir,
		SystemPrompt: agent.SoldatiSystemPrompt,
		MCPConfig:    mcpConfigPath,
		Model:        "sonnet", // Default to sonnet for cost efficiency
	})
	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}

	// Register in registry
	record := &registry.AgentRecord{
		ID:        a.ID,
		Type:      "soldati",
		Name:      name,
		Turf:      d.mobDir, // Default turf to mob directory, updated when work is assigned
		Status:    "idle",
		StartedAt: a.StartedAt,
		LastPing:  time.Now(),
	}
	if err := d.registry.Register(record); err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	// Keep reference in memory
	d.mu.Lock()
	d.activeAgents[name] = a
	d.mu.Unlock()

	// Set up hook watching for this soldati
	if err := d.startHookWatcher(name, a); err != nil {
		d.logger.Printf("Patrol: warning - failed to start hook watcher for '%s': %v\n", name, err)
	}

	d.logger.Printf("Patrol: soldati '%s' is now active (ID: %s)\n", name, a.ID)
	return nil
}

// startHookWatcher begins watching the hook file for a soldati
func (d *Daemon) startHookWatcher(name string, a *agent.Agent) error {
	// Create hook manager
	hookDir := filepath.Join(d.mobDir, ".mob", "soldati")
	mgr, err := hook.NewManager(hookDir, name)
	if err != nil {
		return fmt.Errorf("failed to create hook manager: %w", err)
	}

	// Create cancellable context for this watcher
	ctx, cancel := context.WithCancel(d.ctx)

	// Start watching
	hookChan, err := mgr.Watch(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start hook watcher: %w", err)
	}

	// Store references
	d.mu.Lock()
	d.hookManagers[name] = mgr
	d.hookCancels[name] = cancel
	d.mu.Unlock()

	// Start goroutine to process hooks
	go d.processHooks(name, a, hookChan, mgr)

	d.logger.Printf("Patrol: hook watcher started for soldati '%s'\n", name)
	return nil
}

// processHooks handles incoming hook messages for a soldati
func (d *Daemon) processHooks(name string, a *agent.Agent, hookChan <-chan *hook.Hook, mgr *hook.Manager) {
	for h := range hookChan {
		switch h.Type {
		case hook.HookTypeAssign:
			d.handleAssignment(name, a, h, mgr)
		case hook.HookTypeNudge:
			d.logger.Printf("Hook: nudge received for soldati '%s'\n", name)
			// Nudge just wakes up the agent - no action needed with per-call model
		case hook.HookTypeAbort:
			d.logger.Printf("Hook: abort received for soldati '%s'\n", name)
			// With per-call model, we can't abort mid-execution
			// Just clear the hook and mark idle
			mgr.Clear()
			d.registry.UpdateStatus(a.ID, "idle")
		case hook.HookTypePause:
			d.logger.Printf("Hook: pause received for soldati '%s'\n", name)
			d.registry.UpdateStatus(a.ID, "paused")
		case hook.HookTypeResume:
			d.logger.Printf("Hook: resume received for soldati '%s'\n", name)
			d.registry.UpdateStatus(a.ID, "idle")
		}
	}
}

// handleAssignment processes a work assignment for a soldati
func (d *Daemon) handleAssignment(name string, a *agent.Agent, h *hook.Hook, mgr *hook.Manager) {
	d.logger.Printf("Hook: work assignment for soldati '%s': bead=%s\n", name, h.BeadID)

	// Update status to working
	d.registry.UpdateStatus(a.ID, "active")
	d.registry.UpdateTask(a.ID, h.Message)

	// Execute the work via Chat
	go func() {
		// Build the task message
		taskMsg := h.Message
		if h.BeadID != "" {
			taskMsg = fmt.Sprintf("[Bead %s] %s", h.BeadID, h.Message)
		}

		d.logger.Printf("Soldati '%s' starting work: %s\n", name, truncateMessage(taskMsg, 80))

		// Call the agent
		resp, err := a.Chat(taskMsg)
		if err != nil {
			d.logger.Printf("Soldati '%s' error: %v\n", name, err)
			d.registry.UpdateStatus(a.ID, "error")
			return
		}

		// Log completion
		responseText := resp.GetText()
		d.logger.Printf("Soldati '%s' completed work. Response: %s\n", name, truncateMessage(responseText, 200))

		// Clear the hook and mark idle
		mgr.Clear()
		d.registry.UpdateStatus(a.ID, "idle")
		d.registry.UpdateTask(a.ID, "")
		d.registry.Ping(a.ID)
	}()
}

// truncateMessage truncates a message for logging
func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}

// checkAgentHealth monitors an active agent and restarts if needed
func (d *Daemon) checkAgentHealth(name string, record *registry.AgentRecord) {
	d.mu.RLock()
	a, ok := d.activeAgents[name]
	d.mu.RUnlock()

	if !ok {
		// Agent in registry but not in memory - this can happen after daemon restart
		// Try to respawn the agent directly instead of removing it
		d.logger.Printf("Patrol: soldati '%s' in registry but not in memory, respawning...\n", name)

		// Check if soldati TOML exists before respawning
		soldatiPath := filepath.Join(d.mobDir, "soldati", name+".toml")
		if _, err := os.Stat(soldatiPath); os.IsNotExist(err) {
			// No TOML file - this soldati was never properly set up, remove it
			d.logger.Printf("Patrol: soldati '%s' has no TOML file, removing from registry\n", name)
			d.registry.Unregister(record.ID)
			return
		}

		// Respawn the agent and update the registry with the new process
		if err := d.respawnSoldati(name, record); err != nil {
			d.logger.Printf("Patrol: failed to respawn soldati '%s': %v\n", name, err)
			// Don't unregister on failure - leave it for next patrol cycle
		}
		return
	}

	// Check if agent process is still running
	if !a.IsRunning() {
		d.logger.Printf("Patrol: soldati '%s' process not running, removing from registry\n", name)
		d.registry.Unregister(record.ID)
		d.stopHookWatcher(name)
		d.mu.Lock()
		delete(d.activeAgents, name)
		d.mu.Unlock()
		// Will be respawned on next patrol
		return
	}

	// Update last ping
	d.registry.Ping(record.ID)
}

// resolveTurfPath resolves a turf name to its actual filesystem path
func (d *Daemon) resolveTurfPath(turfName string) string {
	if turfName == "" {
		return d.mobDir
	}
	// If it looks like an absolute path already, use it directly
	if filepath.IsAbs(turfName) {
		return turfName
	}
	// Try to resolve via turf manager
	if d.turfMgr != nil {
		if t, err := d.turfMgr.Get(turfName); err == nil {
			return t.Path
		}
	}
	// Fallback to mob directory
	return d.mobDir
}

// respawnSoldati recreates an agent process for an existing registry entry
func (d *Daemon) respawnSoldati(name string, record *registry.AgentRecord) error {
	workDir := d.resolveTurfPath(record.Turf)

	// Generate MCP config for tool access
	mcpConfigPath, err := mcp.GenerateMCPConfig(d.mobDir)
	if err != nil {
		d.logger.Printf("Warning: failed to generate MCP config: %v", err)
	}

	// Spawn a new agent process
	a, err := d.spawner.SpawnWithOptions(agent.SpawnOptions{
		Type:         agent.AgentTypeSoldati,
		Name:         name,
		Turf:         record.Turf,
		WorkDir:      workDir,
		SystemPrompt: agent.SoldatiSystemPrompt,
		MCPConfig:    mcpConfigPath,
		Model:        "sonnet", // Default to sonnet for cost efficiency
	})
	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}

	// Update registry with new process info (keep existing ID for continuity)
	record.StartedAt = a.StartedAt
	record.LastPing = time.Now()
	record.Status = "idle"
	if err := d.registry.Register(record); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	// Keep reference in memory
	d.mu.Lock()
	d.activeAgents[name] = a
	d.mu.Unlock()

	// Set up hook watching
	if err := d.startHookWatcher(name, a); err != nil {
		d.logger.Printf("Patrol: warning - failed to start hook watcher for '%s': %v\n", name, err)
	}

	d.logger.Printf("Patrol: respawned soldati '%s' (ID: %s)\n", name, record.ID)
	return nil
}

// stopHookWatcher stops the hook watcher for a soldati
func (d *Daemon) stopHookWatcher(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if cancel, ok := d.hookCancels[name]; ok {
		cancel()
		delete(d.hookCancels, name)
	}
	delete(d.hookManagers, name)
}

// AssignWork assigns work to a soldati via their hook file
func (d *Daemon) AssignWork(name, beadID, message string) error {
	d.mu.RLock()
	mgr, ok := d.hookManagers[name]
	d.mu.RUnlock()

	if !ok {
		// Try to create a new hook manager if soldati exists but no watcher
		hookDir := filepath.Join(d.mobDir, ".mob", "soldati")
		var err error
		mgr, err = hook.NewManager(hookDir, name)
		if err != nil {
			return fmt.Errorf("soldati '%s' not found or hook manager error: %w", name, err)
		}
	}

	h := &hook.Hook{
		Type:      hook.HookTypeAssign,
		BeadID:    beadID,
		Message:   message,
		Timestamp: time.Now(),
	}

	if err := mgr.Write(h); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}

	return nil
}

// GetHookManager returns the hook manager for a soldati (for external use)
func (d *Daemon) GetHookManager(name string) (*hook.Manager, error) {
	d.mu.RLock()
	mgr, ok := d.hookManagers[name]
	d.mu.RUnlock()

	if ok {
		return mgr, nil
	}

	// Create a new one if daemon isn't tracking it
	hookDir := filepath.Join(d.mobDir, ".mob", "soldati")
	return hook.NewManager(hookDir, name)
}
