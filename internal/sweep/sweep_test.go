package sweep

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
)

func TestSweeper_Review(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock turf directory with some code files
	turfPath := filepath.Join(tmpDir, "turf")
	if err := os.MkdirAll(turfPath, 0755); err != nil {
		t.Fatalf("failed to create turf dir: %v", err)
	}

	// Create a test file with a style issue (commented out code)
	testFile := filepath.Join(turfPath, "main.go")
	testCode := `package main

func main() {
	// TODO: implement this
	// FIXME: this is broken
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Initialize git repo for the turf
	initGitRepo(t, turfPath)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper
	sweeper := New(turfPath, beadStore)

	// Run review sweep
	ctx := context.Background()
	result, err := sweeper.Review(ctx)
	if err != nil {
		t.Fatalf("Review() returned error: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != SweepTypeReview {
		t.Errorf("expected type %q, got %q", SweepTypeReview, result.Type)
	}
	if result.Turf != turfPath {
		t.Errorf("expected turf %q, got %q", turfPath, result.Turf)
	}
	if result.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
	if result.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}
	if result.CompletedAt.Before(result.StartedAt) {
		t.Error("CompletedAt should be after StartedAt")
	}
}

func TestSweeper_Bugs(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock turf directory with code containing bugs markers
	turfPath := filepath.Join(tmpDir, "turf")
	if err := os.MkdirAll(turfPath, 0755); err != nil {
		t.Fatalf("failed to create turf dir: %v", err)
	}

	// Create files with TODO/FIXME/HACK comments
	testFiles := map[string]string{
		"main.go": `package main

func main() {
	// TODO: implement proper error handling
	println("hello")
}
`,
		"util.go": `package main

// FIXME: this function is buggy
func helper() {
	// HACK: temporary workaround
}
`,
	}

	for name, content := range testFiles {
		path := filepath.Join(turfPath, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	// Initialize git repo
	initGitRepo(t, turfPath)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper
	sweeper := New(turfPath, beadStore)

	// Run bugs sweep
	ctx := context.Background()
	result, err := sweeper.Bugs(ctx)
	if err != nil {
		t.Fatalf("Bugs() returned error: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != SweepTypeBugs {
		t.Errorf("expected type %q, got %q", SweepTypeBugs, result.Type)
	}
	if result.ItemsFound < 3 {
		t.Errorf("expected at least 3 items (TODO, FIXME, HACK), got %d", result.ItemsFound)
	}

	// Verify beads were created
	if len(result.Beads) == 0 {
		t.Error("expected beads to be created for found issues")
	}

	// Verify the beads exist in the store
	beads, err := beadStore.List(storage.BeadFilter{Turf: turfPath})
	if err != nil {
		t.Fatalf("failed to list beads: %v", err)
	}
	if len(beads) == 0 {
		t.Error("expected beads in store")
	}
}

func TestSweeper_All(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock turf directory
	turfPath := filepath.Join(tmpDir, "turf")
	if err := os.MkdirAll(turfPath, 0755); err != nil {
		t.Fatalf("failed to create turf dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(turfPath, "main.go")
	testCode := `package main

func main() {
	// TODO: fix this
}
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Initialize git repo
	initGitRepo(t, turfPath)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper
	sweeper := New(turfPath, beadStore)

	// Run all sweeps
	ctx := context.Background()
	results, err := sweeper.All(ctx)
	if err != nil {
		t.Fatalf("All() returned error: %v", err)
	}

	// Verify we got results for both sweep types
	if len(results) != 2 {
		t.Errorf("expected 2 results (review and bugs), got %d", len(results))
	}

	// Verify we have both sweep types
	hasReview := false
	hasBugs := false
	for _, r := range results {
		if r.Type == SweepTypeReview {
			hasReview = true
		}
		if r.Type == SweepTypeBugs {
			hasBugs = true
		}
	}
	if !hasReview {
		t.Error("expected review sweep result")
	}
	if !hasBugs {
		t.Error("expected bugs sweep result")
	}
}

func TestSweepResult(t *testing.T) {
	// Test that SweepResult has correct structure
	now := time.Now()
	result := &SweepResult{
		Type:        SweepTypeReview,
		Turf:        "/path/to/turf",
		StartedAt:   now,
		CompletedAt: now.Add(time.Second),
		ItemsFound:  5,
		Beads:       []string{"bd-1234", "bd-5678"},
		Summary:     "Found 5 issues",
	}

	if result.Type != SweepTypeReview {
		t.Errorf("expected Type SweepTypeReview, got %v", result.Type)
	}
	if result.Turf != "/path/to/turf" {
		t.Errorf("expected Turf '/path/to/turf', got %v", result.Turf)
	}
	if result.ItemsFound != 5 {
		t.Errorf("expected ItemsFound 5, got %v", result.ItemsFound)
	}
	if len(result.Beads) != 2 {
		t.Errorf("expected 2 beads, got %d", len(result.Beads))
	}
	if result.Summary != "Found 5 issues" {
		t.Errorf("expected Summary 'Found 5 issues', got %v", result.Summary)
	}
}

func TestSweepType_Constants(t *testing.T) {
	// Verify sweep type constants
	if SweepTypeReview != "review" {
		t.Errorf("expected SweepTypeReview to be 'review', got %q", SweepTypeReview)
	}
	if SweepTypeBugs != "bugs" {
		t.Errorf("expected SweepTypeBugs to be 'bugs', got %q", SweepTypeBugs)
	}
	if SweepTypeAll != "all" {
		t.Errorf("expected SweepTypeAll to be 'all', got %q", SweepTypeAll)
	}
}

func TestSweeper_Review_NoGitRepo(t *testing.T) {
	// Create temp directory without git
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper for non-git directory
	sweeper := New(tmpDir, beadStore)

	// Review sweep should handle non-git directory gracefully
	ctx := context.Background()
	result, err := sweeper.Review(ctx)
	// Should not error, just return with no items found from git analysis
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even for non-git directory")
	}
}

func TestSweeper_Bugs_EmptyDirectory(t *testing.T) {
	// Create temp directory with no code files
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper for empty directory
	sweeper := New(tmpDir, beadStore)

	// Bug sweep should handle empty directory gracefully
	ctx := context.Background()
	result, err := sweeper.Bugs(ctx)
	if err != nil {
		t.Fatalf("Bugs() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ItemsFound != 0 {
		t.Errorf("expected 0 items in empty directory, got %d", result.ItemsFound)
	}
}

func TestSweeper_CreatesCorrectBeadTypes(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "sweep-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock turf directory
	turfPath := filepath.Join(tmpDir, "turf")
	if err := os.MkdirAll(turfPath, 0755); err != nil {
		t.Fatalf("failed to create turf dir: %v", err)
	}

	// Create files with different markers
	testFile := filepath.Join(turfPath, "code.go")
	testCode := `package main

// TODO: add feature
// FIXME: fix this bug
// HACK: temporary solution
`
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Initialize git repo
	initGitRepo(t, turfPath)

	// Create bead store
	beadDir := filepath.Join(tmpDir, "beads")
	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		t.Fatalf("failed to create bead store: %v", err)
	}

	// Create sweeper and run bugs sweep
	sweeper := New(turfPath, beadStore)
	ctx := context.Background()
	_, err = sweeper.Bugs(ctx)
	if err != nil {
		t.Fatalf("Bugs() returned error: %v", err)
	}

	// Verify beads were created with correct types
	beads, err := beadStore.List(storage.BeadFilter{})
	if err != nil {
		t.Fatalf("failed to list beads: %v", err)
	}

	// Should have beads for the issues found
	hasBug := false
	hasTask := false
	for _, b := range beads {
		if b.Type == models.BeadTypeBug {
			hasBug = true
		}
		if b.Type == models.BeadTypeTask || b.Type == models.BeadTypeChore {
			hasTask = true
		}
	}

	if !hasBug && !hasTask {
		t.Error("expected at least one bead with bug or task type")
	}
}

// Helper function to initialize a git repo in a directory
func initGitRepo(t *testing.T, path string) {
	t.Helper()

	// Initialize git repo
	cmd := []string{"git", "init"}
	if err := runGitCommand(path, cmd...); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	if err := runGitCommand(path, "git", "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to set git email: %v", err)
	}
	if err := runGitCommand(path, "git", "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to set git name: %v", err)
	}

	// Add and commit files
	if err := runGitCommand(path, "git", "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCommand(path, "git", "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}
}

// runGitCommand runs a command in a directory
func runGitCommand(dir string, args ...string) error {
	cmd := newExecCommand(args[0], args[1:]...)
	cmd.Dir = dir
	return cmd.Run()
}
