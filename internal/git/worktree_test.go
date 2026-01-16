package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "mob-git-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to configure git user: %v", err)
	}

	// Create initial commit (required for worktrees)
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to stage files: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir
}

func TestNewWorktreeManager(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	t.Run("valid git repo", func(t *testing.T) {
		manager, err := NewWorktreeManager(tmpDir)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if manager == nil {
			t.Fatal("expected manager, got nil")
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := NewWorktreeManager("/nonexistent/path")
		if err == nil {
			t.Error("expected error for non-existent path, got nil")
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		notGitDir, err := os.MkdirTemp("", "not-git")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(notGitDir)

		_, err = NewWorktreeManager(notGitDir)
		if err == nil {
			t.Error("expected error for non-git directory, got nil")
		}
	})
}

func TestWorktreeManager_Create(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	manager, err := NewWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("creates worktree and branch", func(t *testing.T) {
		beadID := "bd-test123"
		worktree, err := manager.Create(beadID)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Verify worktree path
		expectedPath := filepath.Join(tmpDir, ".mob-worktrees", beadID)
		if worktree.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, worktree.Path)
		}

		// Verify branch name
		expectedBranch := BranchPrefix + beadID
		if worktree.Branch != expectedBranch {
			t.Errorf("expected branch %s, got %s", expectedBranch, worktree.Branch)
		}

		// Verify bead ID
		if worktree.BeadID != beadID {
			t.Errorf("expected beadID %s, got %s", beadID, worktree.BeadID)
		}

		// Verify worktree directory exists
		if _, err := os.Stat(worktree.Path); os.IsNotExist(err) {
			t.Errorf("worktree directory does not exist: %s", worktree.Path)
		}

		// Verify branch was created
		cmd := exec.Command("git", "branch", "--list", expectedBranch)
		cmd.Dir = tmpDir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to list branches: %v", err)
		}
		if !strings.Contains(string(output), expectedBranch) {
			t.Errorf("branch %s was not created", expectedBranch)
		}
	})

	t.Run("duplicate bead ID fails", func(t *testing.T) {
		beadID := "bd-duplicate"
		_, err := manager.Create(beadID)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}

		_, err = manager.Create(beadID)
		if err == nil {
			t.Error("expected error for duplicate bead ID, got nil")
		}
	})
}

func TestWorktreeManager_Get(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	manager, err := NewWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("get existing worktree", func(t *testing.T) {
		beadID := "bd-gettest"
		created, err := manager.Create(beadID)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		retrieved, err := manager.Get(beadID)
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		if retrieved.Path != created.Path {
			t.Errorf("expected path %s, got %s", created.Path, retrieved.Path)
		}
		if retrieved.Branch != created.Branch {
			t.Errorf("expected branch %s, got %s", created.Branch, retrieved.Branch)
		}
	})

	t.Run("get non-existent worktree", func(t *testing.T) {
		_, err := manager.Get("bd-nonexistent")
		if err == nil {
			t.Error("expected error for non-existent worktree, got nil")
		}
	})
}

func TestWorktreeManager_List(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	manager, err := NewWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create a few worktrees
	beadIDs := []string{"bd-list1", "bd-list2", "bd-list3"}
	for _, beadID := range beadIDs {
		if _, err := manager.Create(beadID); err != nil {
			t.Fatalf("failed to create worktree %s: %v", beadID, err)
		}
	}

	t.Run("lists only mob worktrees", func(t *testing.T) {
		worktrees, err := manager.List()
		if err != nil {
			t.Fatalf("failed to list worktrees: %v", err)
		}

		if len(worktrees) != len(beadIDs) {
			t.Errorf("expected %d worktrees, got %d", len(beadIDs), len(worktrees))
		}

		// Verify all have mob/ prefix
		for _, wt := range worktrees {
			if !strings.HasPrefix(wt.Branch, BranchPrefix) {
				t.Errorf("worktree branch %s does not have mob/ prefix", wt.Branch)
			}
		}
	})

	t.Run("empty list when no worktrees", func(t *testing.T) {
		// Create a fresh repo
		freshDir := setupTestRepo(t)
		defer os.RemoveAll(freshDir)

		freshManager, err := NewWorktreeManager(freshDir)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		worktrees, err := freshManager.List()
		if err != nil {
			t.Fatalf("failed to list worktrees: %v", err)
		}

		if len(worktrees) != 0 {
			t.Errorf("expected 0 worktrees, got %d", len(worktrees))
		}
	})
}

func TestWorktreeManager_Remove(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	manager, err := NewWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("remove worktree without deleting branch", func(t *testing.T) {
		beadID := "bd-remove1"
		worktree, err := manager.Create(beadID)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		err = manager.Remove(beadID, false)
		if err != nil {
			t.Fatalf("failed to remove worktree: %v", err)
		}

		// Verify worktree directory is gone
		if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
			t.Error("worktree directory should not exist after removal")
		}

		// Verify branch still exists
		cmd := exec.Command("git", "branch", "--list", worktree.Branch)
		cmd.Dir = tmpDir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to list branches: %v", err)
		}
		if !strings.Contains(string(output), worktree.Branch) {
			t.Error("branch should still exist when deleteBranch=false")
		}
	})

	t.Run("remove worktree and delete branch", func(t *testing.T) {
		beadID := "bd-remove2"
		worktree, err := manager.Create(beadID)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		err = manager.Remove(beadID, true)
		if err != nil {
			t.Fatalf("failed to remove worktree: %v", err)
		}

		// Verify worktree directory is gone
		if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
			t.Error("worktree directory should not exist after removal")
		}

		// Verify branch is deleted
		cmd := exec.Command("git", "branch", "--list", worktree.Branch)
		cmd.Dir = tmpDir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to list branches: %v", err)
		}
		if strings.Contains(string(output), worktree.Branch) {
			t.Error("branch should be deleted when deleteBranch=true")
		}
	})

	t.Run("remove non-existent worktree fails", func(t *testing.T) {
		err := manager.Remove("bd-nonexistent", false)
		if err == nil {
			t.Error("expected error when removing non-existent worktree")
		}
	})
}

func TestWorktreeManager_GetMainBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	manager, err := NewWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	mainBranch, err := manager.GetMainBranch()
	if err != nil {
		t.Fatalf("failed to get main branch: %v", err)
	}

	// Should be either "main" or "master"
	if mainBranch != "main" && mainBranch != "master" {
		t.Errorf("expected main branch to be 'main' or 'master', got '%s'", mainBranch)
	}
}

func TestValidateBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		expected bool
	}{
		{"valid mob branch", "mob/bd-abc123", true},
		{"valid mob branch with longer id", "mob/bd-a1b2c3d4e5", true},
		{"mob prefix only", "mob/", false},
		{"no mob prefix", "feature/something", false},
		{"main branch", "main", false},
		{"master branch", "master", false},
		{"empty string", "", false},
		{"partial mob prefix", "mo/something", false},
		{"mob without slash", "mobsomething", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBranch(tt.branch)
			if result != tt.expected {
				t.Errorf("ValidateBranch(%q) = %v, expected %v", tt.branch, result, tt.expected)
			}
		})
	}
}
