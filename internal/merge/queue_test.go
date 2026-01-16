package merge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "mob-merge-test")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git name: %v", err)
	}

	// Create initial commit on main
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git commit: %v", err)
	}

	// Rename branch to main (in case default is master)
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to rename branch to main: %v", err)
	}

	return tmpDir
}

// createTestBranch creates a branch with a file change for testing merges
func createTestBranch(t *testing.T, repoPath, branchName, fileName, content string) {
	t.Helper()

	// Create and checkout branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch %s: %v", branchName, err)
	}

	// Create/modify file
	filePath := filepath.Join(repoPath, fileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", fileName, err)
	}

	// Add and commit
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Add "+fileName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Switch back to main
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
}

func TestQueue_Add(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Test adding a single item
	err := q.Add("bd-001", "mob/bd-001", "frontend", nil)
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	items := q.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].BeadID != "bd-001" {
		t.Errorf("expected BeadID 'bd-001', got '%s'", items[0].BeadID)
	}
	if items[0].Branch != "mob/bd-001" {
		t.Errorf("expected Branch 'mob/bd-001', got '%s'", items[0].Branch)
	}
	if items[0].Turf != "frontend" {
		t.Errorf("expected Turf 'frontend', got '%s'", items[0].Turf)
	}
	if items[0].Status != StatusPending {
		t.Errorf("expected Status 'pending', got '%s'", items[0].Status)
	}

	// Test adding item with blockers
	err = q.Add("bd-002", "mob/bd-002", "backend", []string{"bd-001"})
	if err != nil {
		t.Fatalf("failed to add item with blockers: %v", err)
	}

	items = q.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Find the second item
	var item2 *QueueItem
	for _, item := range items {
		if item.BeadID == "bd-002" {
			item2 = item
			break
		}
	}
	if item2 == nil {
		t.Fatal("could not find item bd-002")
	}

	if len(item2.BlockedBy) != 1 || item2.BlockedBy[0] != "bd-001" {
		t.Errorf("expected BlockedBy ['bd-001'], got %v", item2.BlockedBy)
	}

	// Test adding duplicate item
	err = q.Add("bd-001", "mob/bd-001", "frontend", nil)
	if err == nil {
		t.Error("expected error when adding duplicate item, got nil")
	}
}

func TestQueue_Remove(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Add some items
	q.Add("bd-001", "mob/bd-001", "frontend", nil)
	q.Add("bd-002", "mob/bd-002", "backend", nil)

	// Remove first item
	err := q.Remove("bd-001")
	if err != nil {
		t.Fatalf("failed to remove item: %v", err)
	}

	items := q.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 item after removal, got %d", len(items))
	}

	if items[0].BeadID != "bd-002" {
		t.Errorf("expected remaining item to be 'bd-002', got '%s'", items[0].BeadID)
	}

	// Test removing non-existent item
	err = q.Remove("bd-999")
	if err == nil {
		t.Error("expected error when removing non-existent item, got nil")
	}
}

func TestQueue_List(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Empty queue
	items := q.List()
	if len(items) != 0 {
		t.Errorf("expected empty list, got %d items", len(items))
	}

	// Add items
	q.Add("bd-001", "mob/bd-001", "frontend", nil)
	q.Add("bd-002", "mob/bd-002", "backend", nil)
	q.Add("bd-003", "mob/bd-003", "frontend", nil)

	items = q.List()
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Verify items are returned (order may vary based on implementation)
	beadIDs := make(map[string]bool)
	for _, item := range items {
		beadIDs[item.BeadID] = true
	}

	for _, id := range []string{"bd-001", "bd-002", "bd-003"} {
		if !beadIDs[id] {
			t.Errorf("expected item %s to be in list", id)
		}
	}
}

func TestQueue_Next_RespectsBlockers(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Add item that is blocked
	q.Add("bd-002", "mob/bd-002", "backend", []string{"bd-001"})

	// Next should return nil because bd-002 is blocked by bd-001
	next := q.Next()
	if next != nil {
		t.Errorf("expected nil (blocked item), got %s", next.BeadID)
	}

	// Add the blocker
	q.Add("bd-001", "mob/bd-001", "frontend", nil)

	// Now bd-001 should be next (it has no blockers)
	next = q.Next()
	if next == nil {
		t.Fatal("expected an item, got nil")
	}
	if next.BeadID != "bd-001" {
		t.Errorf("expected 'bd-001', got '%s'", next.BeadID)
	}

	// Add another unblocked item added later
	time.Sleep(10 * time.Millisecond) // Ensure different AddedAt time
	q.Add("bd-003", "mob/bd-003", "frontend", nil)

	// Next should still be bd-001 (added first)
	next = q.Next()
	if next == nil {
		t.Fatal("expected an item, got nil")
	}
	if next.BeadID != "bd-001" {
		t.Errorf("expected 'bd-001' (oldest), got '%s'", next.BeadID)
	}
}

func TestQueue_Next_SkipsMergedBlockers(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Add a blocker first
	q.Add("bd-001", "mob/bd-001", "frontend", nil)

	// Add item blocked by bd-001
	q.Add("bd-002", "mob/bd-002", "backend", []string{"bd-001"})

	// bd-002 should not be next
	next := q.Next()
	if next.BeadID != "bd-001" {
		t.Errorf("expected 'bd-001', got '%s'", next.BeadID)
	}

	// Mark bd-001 as merged
	for _, item := range q.List() {
		if item.BeadID == "bd-001" {
			item.Status = StatusMerged
			break
		}
	}

	// Now bd-002 should be available (blocker is merged)
	next = q.Next()
	if next == nil {
		t.Fatal("expected bd-002 to be available")
	}
	if next.BeadID != "bd-002" {
		t.Errorf("expected 'bd-002', got '%s'", next.BeadID)
	}
}

func TestQueue_ProcessOrder(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create branches for testing
	createTestBranch(t, tmpDir, "mob/bd-001", "file1.txt", "content 1")
	createTestBranch(t, tmpDir, "mob/bd-002", "file2.txt", "content 2")
	createTestBranch(t, tmpDir, "mob/bd-003", "file3.txt", "content 3")

	q := New(tmpDir)

	// Add items with dependency chain: bd-003 depends on bd-002, bd-002 depends on bd-001
	q.Add("bd-003", "mob/bd-003", "frontend", []string{"bd-002"})
	q.Add("bd-002", "mob/bd-002", "frontend", []string{"bd-001"})
	q.Add("bd-001", "mob/bd-001", "frontend", nil)

	// Process first item - should be bd-001
	result, err := q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Message)
	}
	if result.BeadID != "bd-001" {
		t.Errorf("expected first merge to be 'bd-001', got '%s'", result.BeadID)
	}

	// Process second item - should be bd-002
	result, err = q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Message)
	}
	if result.BeadID != "bd-002" {
		t.Errorf("expected second merge to be 'bd-002', got '%s'", result.BeadID)
	}

	// Process third item - should be bd-003
	result, err = q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.Message)
	}
	if result.BeadID != "bd-003" {
		t.Errorf("expected third merge to be 'bd-003', got '%s'", result.BeadID)
	}

	// Process again - should return nil (nothing left)
	result, err = q.Process()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result (empty queue), got %v", result)
	}
}

func TestQueue_Process_Conflict(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create conflicting branches - both modify the same file
	createTestBranch(t, tmpDir, "mob/bd-001", "shared.txt", "content from branch 1")
	createTestBranch(t, tmpDir, "mob/bd-002", "shared.txt", "different content from branch 2")

	q := New(tmpDir)

	q.Add("bd-001", "mob/bd-001", "frontend", nil)
	q.Add("bd-002", "mob/bd-002", "frontend", nil)

	// Process first - should succeed
	result, err := q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected first merge to succeed: %s", result.Message)
	}

	// Process second - should fail with conflict
	result, err = q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}
	if result.Success {
		t.Error("expected merge conflict, but got success")
	}
	if len(result.ConflictFiles) == 0 {
		t.Error("expected conflict files to be reported")
	}

	// Verify item status is now conflict
	for _, item := range q.List() {
		if item.BeadID == "bd-002" {
			if item.Status != StatusConflict {
				t.Errorf("expected status 'conflict', got '%s'", item.Status)
			}
			break
		}
	}
}

func TestQueue_SetCallbacks(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	createTestBranch(t, tmpDir, "mob/bd-001", "file1.txt", "content 1")

	q := New(tmpDir)

	var mergedItems []*QueueItem
	var conflictItems []*QueueItem

	q.SetCallbacks(
		func(item *QueueItem) {
			mergedItems = append(mergedItems, item)
		},
		func(item *QueueItem, result *MergeResult) {
			conflictItems = append(conflictItems, item)
		},
	)

	q.Add("bd-001", "mob/bd-001", "frontend", nil)

	_, err := q.Process()
	if err != nil {
		t.Fatalf("failed to process: %v", err)
	}

	if len(mergedItems) != 1 {
		t.Errorf("expected 1 merged callback, got %d", len(mergedItems))
	}
	if len(conflictItems) != 0 {
		t.Errorf("expected 0 conflict callbacks, got %d", len(conflictItems))
	}
}

func TestQueue_Concurrency(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	q := New(tmpDir)

	// Test concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			beadID := "bd-" + string(rune('a'+n))
			q.Add(beadID, "mob/"+beadID, "test", nil)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	items := q.List()
	if len(items) != 10 {
		t.Errorf("expected 10 items after concurrent adds, got %d", len(items))
	}
}

func TestQueue_StatusTransitions(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	createTestBranch(t, tmpDir, "mob/bd-001", "file1.txt", "content 1")

	q := New(tmpDir)

	q.Add("bd-001", "mob/bd-001", "frontend", nil)

	// Verify initial status
	items := q.List()
	if items[0].Status != StatusPending {
		t.Errorf("expected initial status 'pending', got '%s'", items[0].Status)
	}

	// Process (will change status to merging then merged)
	result, _ := q.Process()
	if !result.Success {
		t.Fatalf("expected successful merge: %s", result.Message)
	}

	// Verify final status
	items = q.List()
	if items[0].Status != StatusMerged {
		t.Errorf("expected final status 'merged', got '%s'", items[0].Status)
	}
}
