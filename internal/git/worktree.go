package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BranchPrefix is the prefix for all mob-managed branches
const BranchPrefix = "mob/"

// WorktreesDir is the directory where worktrees are stored
const WorktreesDir = ".mob-worktrees"

var (
	// ErrNotGitRepo indicates the path is not a git repository
	ErrNotGitRepo = errors.New("not a git repository")
	// ErrWorktreeExists indicates a worktree already exists for the bead
	ErrWorktreeExists = errors.New("worktree already exists")
	// ErrWorktreeNotFound indicates the worktree does not exist
	ErrWorktreeNotFound = errors.New("worktree not found")
)

// WorktreeManager manages git worktrees for beads
type WorktreeManager struct {
	repoPath string // Path to the main repository
}

// Worktree represents a git worktree
type Worktree struct {
	Path      string
	Branch    string
	BeadID    string
	CreatedAt time.Time
}

// NewWorktreeManager creates a new manager for a repository
func NewWorktreeManager(repoPath string) (*WorktreeManager, error) {
	// Check if path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return nil, ErrNotGitRepo
	}

	return &WorktreeManager{
		repoPath: repoPath,
	}, nil
}

// Create creates a new worktree for a bead
func (m *WorktreeManager) Create(beadID string) (*Worktree, error) {
	branch := BranchPrefix + beadID
	worktreePath := filepath.Join(m.repoPath, WorktreesDir, beadID)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		return nil, ErrWorktreeExists
	}

	// Get the main branch to base the new branch on
	mainBranch, err := m.GetMainBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to determine main branch: %w", err)
	}

	// Create worktree directory parent
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create the worktree with a new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath, mainBranch)
	cmd.Dir = m.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	return &Worktree{
		Path:      worktreePath,
		Branch:    branch,
		BeadID:    beadID,
		CreatedAt: time.Now(),
	}, nil
}

// Get returns info about an existing worktree
func (m *WorktreeManager) Get(beadID string) (*Worktree, error) {
	worktreePath := filepath.Join(m.repoPath, WorktreesDir, beadID)
	branch := BranchPrefix + beadID

	// Check if worktree directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil, ErrWorktreeNotFound
	}

	// Verify it's actually a worktree by checking git worktree list
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse the output to verify this worktree exists
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "worktree ") && strings.HasSuffix(line, worktreePath) {
			found = true
			break
		}
	}

	if !found {
		return nil, ErrWorktreeNotFound
	}

	// Get the stat for created time
	info, err := os.Stat(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat worktree: %w", err)
	}

	return &Worktree{
		Path:      worktreePath,
		Branch:    branch,
		BeadID:    beadID,
		CreatedAt: info.ModTime(),
	}, nil
}

// List returns all mob worktrees
func (m *WorktreeManager) List() ([]*Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []*Worktree
	var currentPath string
	var currentBranch string

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			currentBranch = strings.TrimPrefix(line, "branch refs/heads/")
		} else if line == "" && currentPath != "" && currentBranch != "" {
			// End of a worktree entry
			if strings.HasPrefix(currentBranch, BranchPrefix) {
				beadID := strings.TrimPrefix(currentBranch, BranchPrefix)
				worktrees = append(worktrees, &Worktree{
					Path:   currentPath,
					Branch: currentBranch,
					BeadID: beadID,
				})
			}
			currentPath = ""
			currentBranch = ""
		}
	}

	// Handle last entry if not followed by blank line
	if currentPath != "" && currentBranch != "" && strings.HasPrefix(currentBranch, BranchPrefix) {
		beadID := strings.TrimPrefix(currentBranch, BranchPrefix)
		worktrees = append(worktrees, &Worktree{
			Path:   currentPath,
			Branch: currentBranch,
			BeadID: beadID,
		})
	}

	return worktrees, nil
}

// Remove removes a worktree (and optionally its branch)
func (m *WorktreeManager) Remove(beadID string, deleteBranch bool) error {
	worktreePath := filepath.Join(m.repoPath, WorktreesDir, beadID)
	branch := BranchPrefix + beadID

	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return ErrWorktreeNotFound
	}

	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = m.repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
	}

	// Optionally delete the branch
	if deleteBranch {
		cmd = exec.Command("git", "branch", "-D", branch)
		cmd.Dir = m.repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to delete branch: %s: %w", string(output), err)
		}
	}

	return nil
}

// GetMainBranch returns the main branch name (main or master)
func (m *WorktreeManager) GetMainBranch() (string, error) {
	// Try "main" first
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	cmd.Dir = m.repoPath
	if err := cmd.Run(); err == nil {
		return "main", nil
	}

	// Try "master"
	cmd = exec.Command("git", "rev-parse", "--verify", "master")
	cmd.Dir = m.repoPath
	if err := cmd.Run(); err == nil {
		return "master", nil
	}

	// Fallback: get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to determine main branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ValidateBranch checks if a branch name is safe (mob/* prefix)
func ValidateBranch(branch string) bool {
	if branch == "" {
		return false
	}

	if !strings.HasPrefix(branch, BranchPrefix) {
		return false
	}

	// Ensure there's something after the prefix
	suffix := strings.TrimPrefix(branch, BranchPrefix)
	return len(suffix) > 0
}
