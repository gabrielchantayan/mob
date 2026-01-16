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
	WorktreePath   string     `json:"worktree_path,omitempty"` // Path to git worktree for this bead
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
