package models

import "time"

// BeadStatus represents the status of a bead
type BeadStatus string

const (
	BeadStatusOpen            BeadStatus = "open"
	BeadStatusInProgress      BeadStatus = "in_progress"
	BeadStatusBlocked         BeadStatus = "blocked"
	BeadStatusClosed          BeadStatus = "closed"
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

// BeadEventType represents the type of event in bead history
type BeadEventType string

const (
	BeadEventTypeCreated        BeadEventType = "created"
	BeadEventTypeStatusChange   BeadEventType = "status_change"
	BeadEventTypeAssigned       BeadEventType = "assigned"
	BeadEventTypeComment        BeadEventType = "comment"
	BeadEventTypeWorkStarted    BeadEventType = "work_started"
	BeadEventTypeWorkCompleted  BeadEventType = "work_completed"
	BeadEventTypeWorktreeCreate BeadEventType = "worktree_created"
)

// BeadEvent represents a historical event on a bead
type BeadEvent struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Type      BeadEventType `json:"type"`
	Actor     string        `json:"actor"` // agent name or "user"
	From      string        `json:"from,omitempty"`
	To        string        `json:"to,omitempty"`
	Comment   string        `json:"comment,omitempty"`
}

// Bead represents an atomic unit of work
type Bead struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	Status         BeadStatus   `json:"status"`
	Priority       int          `json:"priority"` // 0-4, 0 = highest
	Type           BeadType     `json:"type"`
	Assignee       string       `json:"assignee,omitempty"`
	Labels         string       `json:"labels,omitempty"`
	Turf           string       `json:"turf"`
	Branch         string       `json:"branch,omitempty"`
	WorktreePath   string       `json:"worktree_path,omitempty"` // Path to git worktree for this bead
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	ClosedAt       *time.Time   `json:"closed_at,omitempty"`
	CreatedBy      string       `json:"created_by,omitempty"`
	CloseReason    string       `json:"close_reason,omitempty"`
	ParentID       string       `json:"parent_id,omitempty"`
	Blocks         []string     `json:"blocks,omitempty"`
	Related        []string     `json:"related,omitempty"`
	DiscoveredFrom string       `json:"discovered_from,omitempty"`
	History        []BeadEvent  `json:"history,omitempty"`
}
