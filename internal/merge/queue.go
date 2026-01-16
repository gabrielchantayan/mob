// Package merge provides the merge queue functionality for managing
// dependency-aware serial merging of beads into the main branch.
package merge

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status constants for queue items
const (
	StatusPending  = "pending"
	StatusMerging  = "merging"
	StatusConflict = "conflict"
	StatusFailed   = "failed"
	StatusMerged   = "merged"
)

var (
	// ErrItemExists indicates an item with the same BeadID already exists in the queue
	ErrItemExists = errors.New("item already exists in queue")
	// ErrItemNotFound indicates the item was not found in the queue
	ErrItemNotFound = errors.New("item not found in queue")
)

// QueueItem represents a bead in the merge queue
type QueueItem struct {
	BeadID    string    // Unique identifier for the bead
	Branch    string    // Git branch name (e.g., "mob/bd-001")
	Turf      string    // Project/repository this bead belongs to
	BlockedBy []string  // Bead IDs that must merge first
	AddedAt   time.Time // When the item was added to the queue
	Status    string    // "pending", "merging", "conflict", "failed", "merged"
}

// MergeResult represents the result of a merge attempt
type MergeResult struct {
	Success       bool     // Whether the merge succeeded
	BeadID        string   // ID of the bead that was processed
	Message       string   // Descriptive message about the result
	ConflictFiles []string // Files with conflicts (if any)
}

// Queue manages the merge queue for dependency-aware serial merging
type Queue struct {
	items      []*QueueItem
	repoPath   string
	mu         sync.RWMutex
	onMerged   func(item *QueueItem)
	onConflict func(item *QueueItem, result *MergeResult)
}

// New creates a new merge queue for the given repository path
func New(repoPath string) *Queue {
	return &Queue{
		items:    make([]*QueueItem, 0),
		repoPath: repoPath,
	}
}

// Add adds a bead to the merge queue
// Returns ErrItemExists if an item with the same BeadID already exists
func (q *Queue) Add(beadID, branch, turf string, blockedBy []string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check for duplicates
	for _, item := range q.items {
		if item.BeadID == beadID {
			return ErrItemExists
		}
	}

	item := &QueueItem{
		BeadID:    beadID,
		Branch:    branch,
		Turf:      turf,
		BlockedBy: blockedBy,
		AddedAt:   time.Now(),
		Status:    StatusPending,
	}

	q.items = append(q.items, item)
	return nil
}

// Remove removes a bead from the queue by BeadID
// Returns ErrItemNotFound if the item doesn't exist
func (q *Queue) Remove(beadID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.items {
		if item.BeadID == beadID {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return nil
		}
	}

	return ErrItemNotFound
}

// Next returns the next bead that can be merged (no pending blockers)
// Returns nil if no items are ready or the queue is empty
// A bead is considered blocked if any of its blockers:
// 1. Are in the queue but not yet merged
// 2. Are not in the queue (blocker hasn't been added yet)
func (q *Queue) Next() *QueueItem {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Build a set of merged bead IDs for quick lookup
	mergedIDs := make(map[string]bool)
	for _, item := range q.items {
		if item.Status == StatusMerged {
			mergedIDs[item.BeadID] = true
		}
	}

	// Build a set of all bead IDs in queue
	allIDs := make(map[string]bool)
	for _, item := range q.items {
		allIDs[item.BeadID] = true
	}

	// Find candidates that are pending and have no pending blockers
	var candidates []*QueueItem
	for _, item := range q.items {
		if item.Status != StatusPending {
			continue
		}

		// Check if all blockers have been merged
		blocked := false
		for _, blockerID := range item.BlockedBy {
			// If blocker is not in queue at all, this item is blocked
			// (the blocker needs to be added first)
			if !allIDs[blockerID] {
				blocked = true
				break
			}
			// If blocker is in queue but not merged, this item is blocked
			if !mergedIDs[blockerID] {
				blocked = true
				break
			}
		}

		if !blocked {
			candidates = append(candidates, item)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by AddedAt (oldest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].AddedAt.Before(candidates[j].AddedAt)
	})

	return candidates[0]
}

// List returns all items in the queue
func (q *Queue) List() []*QueueItem {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Return a copy of the slice to prevent external modification
	result := make([]*QueueItem, len(q.items))
	copy(result, q.items)
	return result
}

// Process attempts to merge the next available item
// Returns nil, nil if no items are ready to be merged
func (q *Queue) Process() (*MergeResult, error) {
	next := q.Next()
	if next == nil {
		return nil, nil
	}

	// Update status to merging
	q.mu.Lock()
	for _, item := range q.items {
		if item.BeadID == next.BeadID {
			item.Status = StatusMerging
			break
		}
	}
	q.mu.Unlock()

	// Attempt the merge
	result := q.attemptMerge(next)

	// Update status based on result
	q.mu.Lock()
	for _, item := range q.items {
		if item.BeadID == next.BeadID {
			if result.Success {
				item.Status = StatusMerged
			} else if len(result.ConflictFiles) > 0 {
				item.Status = StatusConflict
			} else {
				item.Status = StatusFailed
			}
			break
		}
	}
	q.mu.Unlock()

	// Call appropriate callback
	if result.Success && q.onMerged != nil {
		q.onMerged(next)
	} else if !result.Success && q.onConflict != nil {
		q.onConflict(next, result)
	}

	return result, nil
}

// SetCallbacks sets merge event callbacks
func (q *Queue) SetCallbacks(onMerged func(*QueueItem), onConflict func(*QueueItem, *MergeResult)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onMerged = onMerged
	q.onConflict = onConflict
}

// attemptMerge performs the actual git merge operation
func (q *Queue) attemptMerge(item *QueueItem) *MergeResult {
	result := &MergeResult{
		BeadID: item.BeadID,
	}

	// First, get the main branch name
	mainBranch := q.getMainBranch()

	// Make sure we're on the main branch
	cmd := exec.Command("git", "checkout", mainBranch)
	cmd.Dir = q.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to checkout %s: %s", mainBranch, string(output))
		return result
	}

	// Attempt the merge
	cmd = exec.Command("git", "merge", item.Branch, "--no-edit")
	cmd.Dir = q.repoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a conflict
		if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "Merge conflict") {
			result.Success = false
			result.Message = "merge conflict detected"
			result.ConflictFiles = q.getConflictFiles()

			// Abort the merge to clean up
			abortCmd := exec.Command("git", "merge", "--abort")
			abortCmd.Dir = q.repoPath
			abortCmd.Run()

			return result
		}

		result.Success = false
		result.Message = fmt.Sprintf("merge failed: %s", string(output))
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("successfully merged %s into %s", item.Branch, mainBranch)
	return result
}

// getMainBranch returns the main branch name (main or master)
func (q *Queue) getMainBranch() string {
	// Try "main" first
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	cmd.Dir = q.repoPath
	if err := cmd.Run(); err == nil {
		return "main"
	}

	// Try "master"
	cmd = exec.Command("git", "rev-parse", "--verify", "master")
	cmd.Dir = q.repoPath
	if err := cmd.Run(); err == nil {
		return "master"
	}

	// Fallback to main
	return "main"
}

// getConflictFiles returns a list of files with merge conflicts
func (q *Queue) getConflictFiles() []string {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = q.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}
