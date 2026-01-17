# Mob - Implementation TODO

Features specified in SPEC.md that are not yet implemented.

## Major Missing Features

### Approval Flow
- [x] `mob approve <bead-id>` command
- [x] `mob reject <bead-id>` command
- [x] `pending_approval` status handling in workflows (blocks assignment until approved)
- [x] Approval gates before Underboss executes plans (via pending_approval status)
- [ ] TUI inline approval/rejection (TUI feature pending)

### Merge Queue
- [x] Dependency-aware serial merging (respects `blocks` relationships)
- [x] Conflict detection and reassignment to Soldati
- [x] CI failure handling (mark bead blocked, notify)
- [x] Integration with bead completion workflow

### Git Worktree Integration
- [x] Auto-create worktree per bead on assignment
- [x] `mob/<bead-id>` branch creation
- [x] Wire `internal/git/` into bead workflow
- [x] Worktree cleanup on bead completion

### Wisps (Ephemeral Beads)
- [ ] `/tmp/mob/` or `~/mob/.mob/tmp/` storage
- [ ] Wisp creation for high-velocity internal orchestration
- [ ] Auto-cleanup after completion
- [ ] Exclude from git tracking

### Associates
- [x] Spawning mechanism from Soldati/Underboss (via spawn_associate MCP tool)
- [ ] `max_per_soldati` config limit enforcement
- [x] Shorter timeouts than Soldati (auto-cleanup on task completion)
- [x] No hook file - inline work assignment (task assigned at spawn)
- [x] Auto-complete/block linked beads based on success/failure

### Notifications
- [x] Terminal notifications via osascript (macOS)
- [x] Summary reports (periodic digest)
- [x] Notification triggers:
  - [x] Task completion
  - [x] Approval requests
  - [x] Errors/stuck agents
  - [x] Rate limit warnings

### Seance (Session Resume)
- [ ] Query previous sessions via `/resume`
- [ ] Conversation history in `history/current.jsonl`
- [ ] Summarization of older sessions in `history/summaries/`
- [ ] Underboss continuity across context recycling

### CLI Commands Missing
- [x] `mob logs [bead-id]` - view work logs
- [x] `mob pause [--hard]` - pause system (graceful or hard)
- [x] `mob resume` - resume from pause
- [x] `mob soldati attach <name>` - attach to session (basic implementation, full stdio attach deferred)
- [x] Short aliases (`m a` = `mob add`, `m s` = `mob status`, etc.)

### TUI Features Missing
- [ ] Dashboard tab
  - [ ] System status (daemon health, active agents, pending approvals)
  - [ ] Recent activity feed
  - [ ] Quick stats across all turfs
- [ ] Beads tab
  - [ ] Kanban board (Open → In Progress → Blocked → Closed)
  - [ ] Filter by turf, assignee, priority, type
  - [ ] Inline approval/rejection
- [ ] View modes
  - [ ] Focused (one turf at a time)
  - [ ] Split (multiple turfs in tiled panes)
  - [ ] Aggregate (all turfs unified)

### Configuration
- [ ] `stuck_timeout` enforcement
- [ ] `approval_required` toggle
- [ ] `history_mode` (full transcript + summaries)
- [ ] `require_review` before merge
- [ ] Rate limit handling/alerting/queueing

### Safety & Security
- [ ] Filesystem sandboxing (restrict to turf directories)
- [ ] Block access to `~/mob/.mob/` sensitive internals
- [ ] Command blacklist enforcement
- [ ] Cross-turf work detection
- [ ] Linked beads for cross-turf coordination

### Underboss Conversation History
- [ ] Persist transcript to `history/current.jsonl`
- [ ] Summarize older sessions
- [ ] Hybrid mode: recent full + older summaries

---

## Partially Implemented

### Patrol Loop
- [x] Basic health monitoring
- [x] Spawn missing agents
- [x] Restart dead agents
- [ ] Escalating nudge sequence (stdin → hook update → kill/restart)
- [ ] Rate limit detection and pause

### Sweeps
- [x] Bug sweep (TODO/FIXME/HACK detection)
- [x] Review sweep (style, security, WIP markers)
- [ ] Associate swarming for parallel review
- [ ] Summary report to Don for approval

### Heresy
- [x] Pattern detection (naming inconsistencies, deprecated patterns)
- [x] Heresy bead creation
- [ ] Inquisition workflow (`mob heresy purge`)
- [ ] Child beads for each affected location
- [ ] Verification sweep after fixes

### Hook Files
- [x] Basic hook file structure
- [x] `soldati assign` command
- [ ] Full integration with daemon patrol loop
- [ ] Hook watching triggers work pickup

---

## Future Considerations (Post-MVP)

Per SPEC.md, these are explicitly out of scope for MVP:

- Multi-user/team mode
- External issue tracker sync (Linear, GitHub Issues)
- Custom Soldati personality/skill profiles
- Web dashboard alternative to TUI
- Linux/Windows support
- Plugin architecture for extensions
