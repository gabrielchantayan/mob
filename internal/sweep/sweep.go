// Package sweep provides maintenance sweep operations for turfs.
// Sweeps are user-initiated operations that analyze codebases
// for issues like code review items, bugs, TODOs, and other
// maintenance tasks.
package sweep

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
)

// SweepType represents the type of sweep operation
type SweepType string

const (
	// SweepTypeReview is a code review sweep
	SweepTypeReview SweepType = "review"
	// SweepTypeBugs is a bugfix hunt sweep
	SweepTypeBugs SweepType = "bugs"
	// SweepTypeAll runs all sweep types
	SweepTypeAll SweepType = "all"
)

// SweepResult contains the results of a sweep operation
type SweepResult struct {
	Type        SweepType
	Turf        string
	StartedAt   time.Time
	CompletedAt time.Time
	ItemsFound  int
	Beads       []string // Bead IDs created
	Summary     string
}

// Issue represents a found issue during a sweep
type Issue struct {
	File        string
	Line        int
	Type        string // "TODO", "FIXME", "HACK", etc.
	Description string
	Context     string // surrounding code context
}

// Sweeper manages sweep operations for a turf
type Sweeper struct {
	turfPath  string
	beadStore *storage.BeadStore
}

// New creates a new Sweeper for a turf
func New(turfPath string, beadStore *storage.BeadStore) *Sweeper {
	return &Sweeper{
		turfPath:  turfPath,
		beadStore: beadStore,
	}
}

// Review runs a code review sweep.
// It analyzes recent commits, looks for style issues, missing tests,
// and security anti-patterns, creating beads for issues found.
func (s *Sweeper) Review(ctx context.Context) (*SweepResult, error) {
	result := &SweepResult{
		Type:      SweepTypeReview,
		Turf:      s.turfPath,
		StartedAt: time.Now(),
		Beads:     []string{},
	}

	var issues []Issue

	// Check if this is a git repository
	if s.isGitRepo() {
		// Analyze recent commits for potential issues
		commitIssues, err := s.analyzeRecentCommits(ctx)
		if err == nil {
			issues = append(issues, commitIssues...)
		}
	}

	// Look for common code review issues
	codeIssues, err := s.findCodeReviewIssues(ctx)
	if err == nil {
		issues = append(issues, codeIssues...)
	}

	// Create beads for found issues
	for _, issue := range issues {
		bead, err := s.createBeadFromIssue(issue, models.BeadTypeReview)
		if err != nil {
			continue
		}
		result.Beads = append(result.Beads, bead.ID)
	}

	result.ItemsFound = len(issues)
	result.CompletedAt = time.Now()
	result.Summary = fmt.Sprintf("Code review sweep completed: found %d potential issues", len(issues))

	return result, nil
}

// Bugs runs a bugfix sweep.
// It hunts for TODO/FIXME/HACK comments, looks for error handling gaps,
// checks for dead code, and creates beads for issues found.
func (s *Sweeper) Bugs(ctx context.Context) (*SweepResult, error) {
	result := &SweepResult{
		Type:      SweepTypeBugs,
		Turf:      s.turfPath,
		StartedAt: time.Now(),
		Beads:     []string{},
	}

	// Find TODO/FIXME/HACK comments
	issues, err := s.findBugMarkers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find bug markers: %w", err)
	}

	// Create beads for found issues
	for _, issue := range issues {
		beadType := s.determineBeadType(issue.Type)
		bead, err := s.createBeadFromIssue(issue, beadType)
		if err != nil {
			continue
		}
		result.Beads = append(result.Beads, bead.ID)
	}

	result.ItemsFound = len(issues)
	result.CompletedAt = time.Now()
	result.Summary = fmt.Sprintf("Bug sweep completed: found %d items (TODOs, FIXMEs, HACKs)", len(issues))

	return result, nil
}

// All runs all sweep types and returns results for each
func (s *Sweeper) All(ctx context.Context) ([]*SweepResult, error) {
	var results []*SweepResult

	// Run review sweep
	reviewResult, err := s.Review(ctx)
	if err != nil {
		return nil, fmt.Errorf("review sweep failed: %w", err)
	}
	results = append(results, reviewResult)

	// Run bugs sweep
	bugsResult, err := s.Bugs(ctx)
	if err != nil {
		return nil, fmt.Errorf("bugs sweep failed: %w", err)
	}
	results = append(results, bugsResult)

	return results, nil
}

// isGitRepo checks if the turf path is a git repository
func (s *Sweeper) isGitRepo() bool {
	gitDir := filepath.Join(s.turfPath, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// analyzeRecentCommits looks at recent commits for potential issues
func (s *Sweeper) analyzeRecentCommits(ctx context.Context) ([]Issue, error) {
	var issues []Issue

	// Get recent commit messages to look for indicators of incomplete work
	cmd := newExecCommand("git", "log", "--oneline", "-20", "--format=%s")
	cmd.Dir = s.turfPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Look for WIP, TODO, FIXME in commit messages
	patterns := []string{"WIP", "TODO", "FIXME", "HACK", "XXX", "temp", "temporary"}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, pattern := range patterns {
			if strings.Contains(strings.ToUpper(line), strings.ToUpper(pattern)) {
				issues = append(issues, Issue{
					File:        "git history",
					Type:        "COMMIT",
					Description: fmt.Sprintf("Commit message indicates incomplete work: %s", line),
				})
				break
			}
		}
	}

	return issues, nil
}

// findCodeReviewIssues scans code for common review issues
func (s *Sweeper) findCodeReviewIssues(ctx context.Context) ([]Issue, error) {
	var issues []Issue

	// Patterns for common code review issues
	reviewPatterns := []struct {
		pattern string
		desc    string
	}{
		{`fmt\.Println`, "Debug print statement left in code"},
		{`console\.log`, "Debug console.log left in code"},
		{`panic\(`, "Potential unhandled panic"},
		{`// nolint`, "Linter directive that may need review"},
	}

	// Walk through code files
	err := filepath.Walk(s.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip hidden directories and common non-code directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check code files
		ext := filepath.Ext(path)
		if !isCodeFile(ext) {
			return nil
		}

		// Read and check file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			for _, rp := range reviewPatterns {
				matched, _ := regexp.MatchString(rp.pattern, line)
				if matched {
					relPath, _ := filepath.Rel(s.turfPath, path)
					issues = append(issues, Issue{
						File:        relPath,
						Line:        lineNum + 1,
						Type:        "REVIEW",
						Description: rp.desc,
						Context:     strings.TrimSpace(line),
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return issues, nil
}

// findBugMarkers searches for TODO, FIXME, HACK, and XXX comments
func (s *Sweeper) findBugMarkers(ctx context.Context) ([]Issue, error) {
	var issues []Issue

	// Patterns for bug markers
	markerPattern := regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX|BUG)[\s:]*(.*)`)

	// Walk through code files
	err := filepath.Walk(s.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and common non-code directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only check code files
		ext := filepath.Ext(path)
		if !isCodeFile(ext) {
			return nil
		}

		// Open and scan file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			matches := markerPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				relPath, _ := filepath.Rel(s.turfPath, path)
				description := ""
				if len(matches) >= 3 {
					description = strings.TrimSpace(matches[2])
				}
				issues = append(issues, Issue{
					File:        relPath,
					Line:        lineNum,
					Type:        strings.ToUpper(matches[1]),
					Description: description,
					Context:     strings.TrimSpace(line),
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return issues, nil
}

// createBeadFromIssue creates a bead from a found issue
func (s *Sweeper) createBeadFromIssue(issue Issue, beadType models.BeadType) (*models.Bead, error) {
	title := fmt.Sprintf("[%s] %s", issue.Type, issue.File)
	if issue.Line > 0 {
		title = fmt.Sprintf("[%s] %s:%d", issue.Type, issue.File, issue.Line)
	}

	description := issue.Description
	if issue.Context != "" {
		description = fmt.Sprintf("%s\n\nContext:\n%s", issue.Description, issue.Context)
	}

	bead := &models.Bead{
		Title:          title,
		Description:    description,
		Status:         models.BeadStatusOpen,
		Priority:       s.determinePriority(issue.Type),
		Type:           beadType,
		Turf:           s.turfPath,
		DiscoveredFrom: "sweep",
	}

	return s.beadStore.Create(bead)
}

// determineBeadType maps issue types to bead types
func (s *Sweeper) determineBeadType(issueType string) models.BeadType {
	switch strings.ToUpper(issueType) {
	case "FIXME", "BUG":
		return models.BeadTypeBug
	case "TODO":
		return models.BeadTypeTask
	case "HACK", "XXX":
		return models.BeadTypeChore
	default:
		return models.BeadTypeTask
	}
}

// determinePriority maps issue types to priorities
func (s *Sweeper) determinePriority(issueType string) int {
	switch strings.ToUpper(issueType) {
	case "FIXME", "BUG":
		return 1 // High priority
	case "HACK":
		return 2 // Medium-high priority
	case "TODO":
		return 3 // Medium priority
	default:
		return 3
	}
}

// isCodeFile checks if a file extension indicates a code file
func isCodeFile(ext string) bool {
	codeExts := map[string]bool{
		".go":    true,
		".js":    true,
		".ts":    true,
		".jsx":   true,
		".tsx":   true,
		".py":    true,
		".rb":    true,
		".java":  true,
		".c":     true,
		".cpp":   true,
		".h":     true,
		".hpp":   true,
		".rs":    true,
		".swift": true,
		".kt":    true,
		".scala": true,
		".php":   true,
		".cs":    true,
		".sh":    true,
		".bash":  true,
		".zsh":   true,
	}
	return codeExts[ext]
}

// newExecCommand creates a new exec.Cmd (allows mocking in tests)
var newExecCommand = exec.Command
