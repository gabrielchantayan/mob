# Mob - Agent Orchestrator Specification

## Overview

**Mob** is a Claude Code agent orchestrator with a mafia-themed naming convention. It enables a single user to manage multiple autonomous Claude Code instances working across multiple projects ("turfs") with persistent worker identities, structured work tracking via JSONL "Beads," and a hybrid CLI/TUI interface.

Inspired by [Gas Town](https://github.com/steveyegge/gastown), Mob adapts its core principles—GUPP (immediate execution), NDI (crash-resilient orchestration), and MEOW (molecular work decomposition)—into a streamlined, single-user system.

## Goals

1. **Autonomous delegation**: Describe what you want conversationally; the Underboss breaks it down and assigns work
2. **Crash-resilient orchestration**: Work continues even when individual Claude sessions fail
3. **Persistent worker identity**: Soldati (named workers) maintain history across sessions
4. **Structured work tracking**: All work tracked as git-backed Beads (JSONL)
5. **Multi-project support**: Manage work across multiple turfs from one central hub
6. **Approval-gated autonomy**: System proposes plans, human approves before execution

## Architecture

### Hierarchy

```
Don (You)
└── Underboss (Chief-of-staff Claude instance)
    ├── Soldati (Named, persistent workers)
    │   └── Associates (Ephemeral, disposable workers)
    └── Associates (Ephemeral workers spawned directly)
```

#### The Don
- The human user
- Interacts via CLI commands or TUI dashboard
- Approves plans proposed by Underboss
- Can attach to any Soldati session for observation or control

#### Underboss
- Persistent Claude Code session (recycled when context fills, maintains continuity via Seance/resume)
- Primary interface for conversational task assignment
- Breaks down requests into Beads, assigns to Soldati
- Proposes plans and waits for Don approval before major actions
- Personality: Efficient and task-focused, but with mob underboss flavor
- Monitors Soldati health, escalates issues

#### Soldati
- Named, persistent workers with tracked history
- Each has a dedicated hook file for work assignment
- Stats tracked: tasks completed, success rate, last active
- Work in isolated git worktrees per Bead
- Can spawn Associates for subtasks
- Seance-capable: can query previous sessions via `/resume`

#### Associates
- Ephemeral, anonymous workers
- No hook file—work assigned inline at spawn
- Shorter timeouts, automatically killed when done
- Limited git access: work on branches, can't merge directly
- No persistent identity or ancestry tracking

### Daemon

A standalone, self-managing daemon (`mobd`) orchestrates all agents:

**Lifecycle:**
- PID file for singleton enforcement
- Watchdog subprocess monitors and restarts if needed
- State file for crash recovery
- Graceful and hard pause modes

**Responsibilities:**
- Spawn/manage Claude Code instances via `claude --dangerously-skip-permissions`
- Communicate via JSON-RPC over stdio
- Monitor agent health via heartbeat/patrol loops
- Escalating nudge: stdin signal → hook file update → kill/restart
- Rate limit handling: alert, queue, pause until reset

**Patrol Loop (idle state):**
- Continuous background patrols even when no active work
- Deacon-equivalent: pings all agents every ~2 minutes
- Boot-equivalent: checks Deacon health every 5 minutes
- Witness-equivalent: per-turf patrol watching active workers

### Communication

**File-Based Hooks:**
- Each Soldati has a single `hook.json` file
- Daemon updates hook file with new work assignments
- Agent polls/watches hook file for changes
- Associates receive work inline at spawn (no hook file)

**IPC Format:**
- JSON-RPC between daemon and Claude Code instances
- Structured requests for control signals
- Natural language for task content within structured envelope

## Data Model

### Beads (Atomic Work Units)

Stored as JSONL in `~/mob/.mob/beads/`

```jsonl
{"id":"bd-a1b2","title":"Add auth middleware","description":"...","status":"in_progress","priority":1,"type":"feature","assignee":"vinnie","labels":"backend,security","created_at":"2024-01-15T10:00:00Z","updated_at":"2024-01-15T10:30:00Z","turf":"project-a","branch":"mob/bd-a1b2"}
```

**Core Fields:**
| Field | Description |
|-------|-------------|
| id | Hash-based ID (e.g., `bd-a1b2`, `bd-f14c.1` for children) |
| title | Short issue title |
| description | Full description |
| status | `open`, `in_progress`, `blocked`, `closed` |
| priority | 0-4 (0 = P0/highest) |
| type | `bug`, `feature`, `task`, `epic`, `chore` |
| assignee | Soldati name or empty |
| labels | Comma-separated tags |
| turf | Project this Bead belongs to |
| created_at, updated_at, closed_at | Timestamps |
| created_by | Creator identifier |
| close_reason | Reason for closure |

**Dependency Links:**
| Type | Meaning |
|------|---------|
| blocks | Hard blocker—can't start until resolved |
| related | Soft connection |
| parent_id | Hierarchical parent |
| discovered_from | Found during work on another Bead |

### Wisps (Ephemeral Beads)

- Stored in `/tmp/mob/` or `~/mob/.mob/tmp/`
- High-velocity internal orchestration work
- Cleaned up after completion
- Never committed to git

### Soldati Profiles

Stored in `~/mob/.mob/soldati/<name>.toml`

```toml
name = "vinnie"
created_at = "2024-01-01T00:00:00Z"
last_active = "2024-01-15T10:30:00Z"

[stats]
tasks_completed = 42
tasks_failed = 3
success_rate = 0.93
```

Minimal context—just name and stats. No personality prompts or skill tags initially.

### Turfs (Projects)

Manually registered via CLI. Stored in `~/mob/turfs.toml`:

```toml
[[turf]]
name = "project-a"
path = "/Users/gabe/Programming/project-a"
main_branch = "main"

[[turf]]
name = "project-b"
path = "/Users/gabe/Programming/project-b"
main_branch = "master"
```

## Directory Structure

```
~/mob/
├── .mob/                    # Internal data (gitignored internals)
│   ├── daemon.pid           # Daemon PID file
│   ├── daemon.state         # Recovery state
│   ├── logs/                # Structured JSON logs
│   │   ├── daemon.log
│   │   ├── underboss.log
│   │   └── soldati/
│   │       └── vinnie.log
│   ├── tmp/                 # Wisps (ephemeral beads)
│   └── soldati/             # Soldati hook files
│       └── vinnie/
│           ├── hook.json
│           └── session/     # Session data for Seance
├── beads/                   # Git-tracked Bead storage
│   ├── open.jsonl
│   ├── closed.jsonl
│   └── archive/
├── soldati/                 # Soldati profiles
│   └── vinnie.toml
├── history/                 # Underboss conversation history
│   ├── current.jsonl        # Recent full transcript
│   └── summaries/           # Older summarized sessions
├── config.toml              # Main configuration
└── turfs.toml               # Registered projects
```

## Interfaces

### CLI (`mob`)

Single binary with subcommands:

**Core Commands:**
```bash
mob init                     # Interactive setup wizard
mob daemon start|stop|status # Daemon control
mob tui                      # Launch TUI dashboard
```

**Conversational (Underboss):**
```bash
mob chat                     # Interactive chat session
mob ask "question"           # One-shot question
mob tell "instruction"       # One-shot command
```

**Task Management:**
```bash
mob add "task description"   # Create a Bead
mob status [bead-id]         # Show status
mob approve <bead-id>        # Approve pending plan
mob reject <bead-id>         # Reject with reason
mob logs [bead-id]           # View work logs
```

**Agent Management:**
```bash
mob soldati list             # List all Soldati
mob soldati new [name]       # Create new Soldati (auto-names if omitted)
mob soldati attach <name>    # Attach to session (observe/message/control)
mob soldati kill <name>      # Terminate a Soldati
mob nudge [soldati|all]      # Nudge stuck agents
```

**Turf Management:**
```bash
mob turf add <path> [name]   # Register a turf
mob turf list                # List turfs
mob turf remove <name>       # Unregister turf
```

**Control:**
```bash
mob pause [--hard]           # Pause system (graceful or hard)
mob resume                   # Resume from pause
```

**Shortcuts:** All commands have short aliases (e.g., `m a` = `mob add`, `m s` = `mob status`)

### TUI (`mob tui`)

Built with Bubbletea. Tabbed interface with multiple views:

**Dashboard Tab:**
- System status (daemon health, active agents, pending approvals)
- Recent activity feed
- Quick stats across all turfs

**Agents Tab:**
- List of Underboss + all Soldati
- Status indicators (active/idle/stuck)
- Current task for each
- Attach keybind to enter agent session

**Beads Tab:**
- Kanban-style board: Open → In Progress → Blocked → Closed
- Filter by turf, assignee, priority, type
- Inline approval/rejection

**Logs Tab:**
- Real-time log stream
- Filter by agent, severity, turf
- Search functionality

**View Modes:**
- Focused: One turf at a time, switch with keybind
- Split: Multiple turfs in tiled panes
- Aggregate: All turfs in unified view

### Notifications

Multi-channel notification system:
- Real-time TUI updates
- Terminal notifications (macOS notifications via osascript)
- Summary reports (periodic digest of activity)

Notification triggers:
- Task completion
- Approval requests
- Errors/stuck agents
- Rate limit warnings

## Workflows

### Task Assignment Flow

1. **Don** tells Underboss what they want (conversational)
2. **Underboss** analyzes request, proposes Bead breakdown
3. **Don** reviews and approves (or modifies) plan
4. **Underboss** creates Beads, assigns to Soldati based on availability
5. **Soldati** receive work via hook file, begin execution
6. Each **Soldati** creates git worktree for their Bead (`mob/bd-xxxx`)
7. Work proceeds; Associates spawned as needed for subtasks
8. **Soldati** submit completed work to merge queue
9. **Merge queue** respects Bead dependencies, merges serially
10. **Underboss** notifies Don of completion

### Approval Flow

When Underboss needs approval:
1. Creates Bead with `status: pending_approval`
2. Notifies Don via all channels
3. Don can approve via:
   - TUI inline approval
   - CLI: `mob approve bd-xxxx`
   - Chat: respond in conversation

### Recovery Flow

When an agent appears stuck:
1. Patrol loop detects stale hook + no recent Bead updates
2. Escalating nudge:
   - Send newline to stdin
   - Update hook file with nudge signal
   - Kill and restart with Seance (resume from previous session)
3. If repeated failures, escalate to Underboss
4. Underboss may reassign to different Soldati or surface to Don

### Merge Queue

Dependency-aware serial merging:
1. Soldati completes work, marks Bead ready
2. Merge queue considers all ready Beads
3. Respects `blocks` dependencies for ordering
4. Attempts merge of next candidate
5. If conflict: reassign to Soldati for resolution
6. If CI fails: mark Bead blocked, notify
7. If success: merge, move to next candidate

## Maintenance Workflows

### Sweeps

User-initiated maintenance operations that run across a turf to maintain code quality. The Don kicks these off when needed—they don't run automatically.

#### Code-Review Sweep

Systematic review of recent changes to catch issues before they compound.

**Trigger:** `mob sweep review [turf]`

**Process:**
1. Underboss identifies commits since last sweep (or specified range)
2. Spawns Associates to review chunks in parallel
3. Each Associate produces review notes as Beads (type: `review`)
4. Issues found become linked Beads (type: `bug` or `chore`)
5. Summary report surfaces to Don for approval

**Review Focus:**
- Code style consistency
- Test coverage gaps
- Security anti-patterns
- Performance concerns
- Documentation staleness

#### Bugfix Sweep

Proactive hunt for bugs, tech debt, and potential issues.

**Trigger:** `mob sweep bugs [turf]`

**Process:**
1. Underboss analyzes turf for common bug patterns
2. Associates swarm across the codebase looking for:
   - TODO/FIXME/HACK comments
   - Error handling gaps
   - Dead code paths
   - Dependency vulnerabilities
   - Test failures or flaky tests
3. Discovered issues filed as Beads with `discovered_from: sweep-<timestamp>`
4. Priority auto-assigned based on severity heuristics
5. Don reviews and approves which bugs to fix

**Output:** Prioritized backlog of bugfix Beads ready for assignment.

### Heresies

**Heresy**: A wrong architectural assumption, anti-pattern, or misconception that has spread through the codebase. Like a disease—the longer it spreads, the harder it is to eradicate.

#### Why Heresies Matter

AI agents (and humans) can propagate mistakes. If an early Soldati makes a wrong assumption—say, using the wrong auth pattern or misunderstanding a data model—subsequent work may copy and amplify that mistake. Without active detection, heresies become load-bearing bugs.

#### Heresy Detection

**Trigger:** `mob heresy scan [turf]`

**Process:**
1. Underboss reviews architectural docs (if present) and established patterns
2. Associates analyze codebase for:
   - Pattern inconsistencies (multiple ways of doing the same thing)
   - Deprecated patterns still in use
   - Copy-paste code that diverged incorrectly
   - Comments that contradict the code
   - Naming inconsistencies suggesting conceptual confusion
3. Suspected heresies filed as Beads (type: `heresy`)
4. Each heresy Bead includes:
   - Description of the wrong assumption
   - Files/locations where it appears
   - Correct pattern (if known)
   - Spread assessment (how many places affected)

#### Heresy Inquisition

When a heresy is confirmed, the **Inquisition** workflow eradicates it:

**Trigger:** `mob heresy purge <heresy-bead-id>`

**Process:**
1. Don approves the heresy diagnosis and correct pattern
2. Underboss creates child Beads for each affected location
3. Soldati work in parallel to:
   - Fix each instance
   - Add tests to prevent regression
   - Update any docs that propagated the heresy
4. Merge queue processes fixes in dependency order
5. Final verification sweep confirms eradication

#### Preventing Heresies

**Architectural Decision Records (ADRs):**
- Store in `<turf>/docs/adr/` or turf-specific location
- Underboss and Soldati reference ADRs before making architectural choices
- New patterns require ADR approval from Don

**Pattern Library:**
- Canonical examples of "the right way" for common operations
- Soldati consult pattern library before implementing

**Early Detection:**
- Code-review sweeps flag pattern inconsistencies
- Underboss tracks "this seems different from before" observations

### Sweep CLI Commands

```bash
mob sweep review [turf]          # Run code-review sweep
mob sweep bugs [turf]            # Run bugfix sweep
mob sweep all [turf]             # Run all sweeps

mob heresy scan [turf]           # Scan for heresies
mob heresy list [turf]           # List known heresies
mob heresy purge <bead-id>       # Eradicate a heresy
```

## Safety & Security

### Git Safety
- Agents work only on `mob/*` branches
- Never push directly to main/master
- Human review gate before merge
- Branch naming: `mob/<bead-id>` (e.g., `mob/bd-a1b2`)

### Filesystem Sandboxing
- Agents restricted to their assigned turf directories
- Cannot access `~/mob/.mob/` sensitive internals
- Cannot access other turfs without explicit cross-turf Bead

### Command Blacklist
Configurable list of forbidden shell commands:
- `rm -rf /`
- `sudo` (unless explicitly allowed per-turf)
- Destructive git operations (`push --force`, etc.)

### Cross-Turf Work
- Automatic detection when work spans turfs
- Creates linked Beads in each affected turf
- Coordinates merge order across turfs

## Configuration

### Main Config (`~/mob/config.toml`)

```toml
[daemon]
heartbeat_interval = "2m"
boot_check_interval = "5m"
stuck_timeout = "10m"
max_concurrent_agents = 5

[underboss]
personality = "efficient mob underboss"
approval_required = true
history_mode = "hybrid"  # full transcript + summaries

[soldati]
auto_name = true  # Generate mob names like "Vinnie", "Sal"
default_timeout = "30m"

[associates]
timeout = "10m"
max_per_soldati = 3

[notifications]
terminal = true
summary_interval = "1h"

[safety]
branch_prefix = "mob/"
command_blacklist = ["sudo", "rm -rf"]
require_review = true

[logging]
level = "info"
format = "dual"  # human terminal + JSON files
retention = "7d"
```

### First-Run Setup

Interactive wizard (`mob init`):
1. Where should mob live? (default: `~/mob`)
2. Register your first turf (project path)
3. Name your first Soldati (or auto-generate)
4. Configure notification preferences
5. Start daemon? (y/n)

## Technical Implementation

### Language & Stack
- **Language**: Go
- **TUI**: Bubbletea
- **CLI**: Cobra + Viper (or similar)
- **Logging**: Zerolog or similar (leveled, dual-output)
- **IPC**: JSON-RPC over stdio to Claude Code

### Single Binary
One executable handles all modes:
- `mob daemon` - runs as daemon
- `mob tui` - launches TUI
- `mob <command>` - CLI operations

### Claude Code Integration
- Spawn via: `claude --dangerously-skip-permissions`
- Communicate via JSON-RPC on stdin/stdout
- Use `/resume` for Seance functionality
- Parse output for health monitoring

### Platform
- **macOS only** (initial release)
- Design for Unix portability (Linux later)
- Uses macOS notifications (`osascript`)

## Terminology Glossary

| Term | Meaning |
|------|---------|
| Don | The human user (you) |
| Underboss | Chief-of-staff Claude instance, primary interface |
| Soldati | Named, persistent worker agents |
| Associates | Ephemeral, disposable worker agents |
| Turf | A project/repository under mob's management |
| Bead | Atomic unit of work (JSONL issue-tracker entry) |
| Wisp | Ephemeral Bead that doesn't persist to git |
| Hook | File-based work assignment mechanism |
| Nudge | Signal to wake up a potentially stuck agent |
| Seance | Querying previous sessions via `/resume` |
| Patrol | Background health-check loop |
| Sweep | User-initiated codebase-wide maintenance operation |
| Heresy | Wrong architectural assumption spreading through codebase |
| Inquisition | Workflow to eradicate a confirmed heresy |

## Open Questions

1. **Soldati specialization**: Should Soldati develop tracked specialties over time (e.g., "good at frontend")? Currently minimal context only.

2. **Associate spawn limits**: What's the right cap on concurrent Associates per Soldati or system-wide?

3. **Bead ID collision**: What hash function/format for Bead IDs? How to handle edge cases?

4. **Cross-turf dependencies**: What's the UX for approving work that spans multiple turfs?

5. **Rate limit prediction**: Should the system try to predict and avoid hitting rate limits proactively?

## Success Metrics

- **Throughput**: Tasks completed per day
- **Reliability**: % of tasks completed without human intervention after approval
- **Recovery**: % of stuck agents successfully recovered automatically
- **Latency**: Time from task approval to Soldati starting work
- **Cost efficiency**: API calls per completed task

## Future Considerations (Not MVP)

- Multi-user/team mode
- External issue tracker sync (Linear, GitHub Issues)
- Custom Soldati personality/skill profiles
- Web dashboard alternative to TUI
- Linux/Windows support
- Plugin architecture for extensions
