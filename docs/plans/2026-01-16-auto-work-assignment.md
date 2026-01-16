# Auto Work Assignment Flow (Gas Town Port)

**Date:** 2026-01-16
**Topic:** Porting the Gas Town "next bead" flow to mob-tools

## Overview

In Gas Town, agents don't self-serve work - it's pushed to them. When an agent finishes a bead, it goes idle and waits. The Mayor (orchestrator) notices via patrol, queries ready beads, picks the next one, slings it to the idle agent, and nudges them to wake up.

This document plans the implementation of this flow in mob-tools.

## Current State (Mob)

**What we have:**
- Hook system (`internal/hook/hook.go`) - writes work assignments to `hook.json` files
- Nudge system (`internal/nudge/nudge.go`) - escalating nudge levels
- Patrol loop (`internal/daemon/daemon.go`) - checks agent health every 2 min, nudges all agents every 5 min
- `assign_bead` MCP tool - writes hook files to assign work
- Daemon processes hooks via `handleAssignment()` - executes work via agent.Chat()
- When work completes, daemon clears hook and sets status to "idle"

**What's missing:**
1. **Ready bead query** - No way to find beads that are ready to work (open + no blockers)
2. **Auto-assignment on idle** - Daemon patrol doesn't auto-assign next bead to idle agents
3. **Turf-scoped work** - Agents should get beads from their assigned turf

## Design

### 1. Ready Beads Query

Add a `ListReady` method to BeadStore that returns beads where:
- Status = "open"
- All beads in `Blocks` array are closed (no blockers)
- Optionally filtered by turf

**File:** `internal/storage/bead_store.go`

```go
// ListReady returns beads that are ready for assignment:
// - Status is "open"
// - All blocking beads (in Blocks array) are closed
// - Sorted by priority (0 = highest first)
func (s *BeadStore) ListReady(turf string) ([]*models.Bead, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    allBeads, err := s.readAllBeads()
    if err != nil {
        return nil, err
    }

    // Build map of closed bead IDs for blocker checking
    closedIDs := make(map[string]bool)
    for _, b := range allBeads {
        if b.Status == models.BeadStatusClosed {
            closedIDs[b.ID] = true
        }
    }

    var ready []*models.Bead
    for _, b := range allBeads {
        // Must be open
        if b.Status != models.BeadStatusOpen {
            continue
        }

        // Turf filter
        if turf != "" && b.Turf != turf {
            continue
        }

        // Check blockers - all must be closed
        allBlockersClosed := true
        for _, blockerID := range b.Blocks {
            if !closedIDs[blockerID] {
                allBlockersClosed = false
                break
            }
        }
        if !allBlockersClosed {
            continue
        }

        ready = append(ready, b)
    }

    // Sort by priority (0 = highest priority, should be first)
    sort.Slice(ready, func(i, j int) bool {
        return ready[i].Priority < ready[j].Priority
    })

    return ready, nil
}
```

### 2. MCP Tool: list_ready_beads

Add a new MCP tool for the Underboss to query ready beads.

**File:** `internal/mcp/tools.go`

```go
{
    Name:        "list_ready_beads",
    Description: "Get beads that are ready to work - open status with no unmet blockers. Returns sorted by priority.",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "turf": map[string]interface{}{
                "type":        "string",
                "description": "Filter by project/territory (optional)",
            },
            "limit": map[string]interface{}{
                "type":        "integer",
                "description": "Maximum number of beads to return (default 10)",
            },
        },
    },
    Handler: handleListReadyBeads,
}
```

### 3. Auto-Assignment in Daemon Patrol

Modify `patrol()` in daemon to check for idle agents and auto-assign next ready bead.

**File:** `internal/daemon/daemon.go`

Add to patrol flow after health checks:

```go
// In patrol(), after checking agent health:
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
```

### 4. DYFJ Signal (Do Your F***ing Job)

Add periodic "DYFJ" nudge to keep agents active, similar to Gas Town's Deacon.

The current `nudgeAllAgents()` already does this every 5 minutes with "Do your job." message.
This is effectively the DYFJ signal. No changes needed.

### 5. Completion Callback (Agent Reports Done)

When an agent marks a bead as complete via `complete_bead`, the next patrol cycle will:
1. Notice the agent's hook is empty
2. Notice the agent is idle
3. Query ready beads for that turf
4. Assign the next one

This happens automatically with the above changes.

## Implementation Order

### Phase 1: Ready Beads Query
1. Add `ListReady()` to `BeadStore` in `internal/storage/bead_store.go`
2. Add test in `internal/storage/bead_store_test.go`
3. Add `list_ready_beads` MCP tool in `internal/mcp/tools.go`

### Phase 2: Daemon Auto-Assignment
1. Add `beadStore` field to Daemon struct
2. Initialize it in `Start()`
3. Add `assignWorkToIdleAgents()` method
4. Call it at end of `patrol()`
5. Add `nudgeAgent()` helper method

### Phase 3: Testing
1. Manual test: Create soldati, create beads, watch auto-assignment
2. Verify bead transitions: open -> in_progress -> closed
3. Verify next bead gets assigned after completion

## Architectural Decisions

### Push Model (Not Pull)

Following Gas Town's architecture, agents DON'T self-serve:
- Agents cannot query for work
- Work is pushed via hooks
- Daemon (orchestrator) has global view and makes scheduling decisions

**Why push over pull?**
1. **Centralized prioritization** - Daemon can load-balance, respect dependencies, prioritize intelligently
2. **Simpler agent logic** - Agents just execute what's on their hook
3. **Matches Claude Code's model** - Claude Code naturally waits for input; push + nudge works with this

### Turf Scoping

Agents work on a specific turf (project). When auto-assigning:
- Only consider beads from the agent's turf
- This prevents cross-project confusion
- Turf is set when soldati is spawned or assigned

### Priority Ordering

Ready beads are sorted by priority (0 = highest):
- Critical (0) gets assigned first
- Lowest (4) gets assigned last
- Within same priority, FIFO based on creation time

### Blocker Handling

A bead is "ready" only when all its blockers are closed:
- `Blocks` array lists bead IDs that must complete first
- If any blocker is not closed, the bead is not ready
- This respects dependency ordering

## Files to Modify

| File | Change |
|------|--------|
| `internal/storage/bead_store.go` | Add `ListReady()` method |
| `internal/storage/bead_store_test.go` | Add tests for `ListReady()` |
| `internal/mcp/tools.go` | Add `list_ready_beads` tool |
| `internal/daemon/daemon.go` | Add `beadStore` field, `assignWorkToIdleAgents()`, call in `patrol()` |

## Flow Diagram

```
Agent completes bead
        │
        ▼
calls complete_bead()
        │
        ▼
bead status → closed
        │
        ▼
daemon.handleAssignment() returns
        │
        ▼
hook cleared, status → idle
        │
        ▼
    [WAIT]
        │
        ▼
daemon patrol() (every 2 min)
        │
        ▼
assignWorkToIdleAgents()
        │
        ├── agent.Status == "idle"?
        │           │
        │           ▼ yes
        ├── hook empty?
        │           │
        │           ▼ yes
        ├── ListReady(turf)
        │           │
        │           ▼ beads found
        ├── pick first (highest priority)
        │           │
        │           ▼
        ├── AssignWork() → writes hook
        │           │
        │           ▼
        └── nudgeAgent() → "Check your hook"
                    │
                    ▼
        Agent wakes, reads hook, executes work
```

## Edge Cases

1. **No ready beads** - Agent stays idle, next patrol will check again
2. **All agents busy** - Beads queue up, assigned when agents become idle
3. **Agent crashes** - Patrol detects dead agent, respawns, then assigns work
4. **Bead has blockers** - Not returned by ListReady until blockers close
5. **Multiple idle agents** - Each gets assigned the next highest priority bead for their turf

## Verification

After implementation, verify:
1. `mob add "Task A" --turf=myproject` creates open bead
2. `mob soldati new` + daemon running spawns idle soldati
3. Patrol auto-assigns Task A to soldati
4. Soldati executes, completes, goes idle
5. Next ready bead (if any) gets auto-assigned

## Comparison to Gas Town

| Concept | Gas Town | Mob |
|---------|----------|-----|
| Sling work | `gt sling <bead> <agent>` | `AssignWork()` → hook file |
| Hook file | `~/.gas-town/hooks/<agent>/hook.json` | `~/.mob/.mob/soldati/<name>/hook.json` |
| Nudge | `gt nudge <agent>` | `a.Chat("Check your hook")` |
| DYFJ signal | Deacon pings Mayor every 2 min | `nudgeAllAgents()` every 5 min |
| Ready query | `bd ready` | `BeadStore.ListReady()` |
| Mayor patrol | Oversees convoy, assigns work | Daemon `patrol()` + `assignWorkToIdleAgents()` |
