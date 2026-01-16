package display

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
)

// TreeOpts configures tree rendering
type TreeOpts struct {
	ShowStatus   bool
	ShowPriority bool
	ColorEnabled bool
	MaxDepth     int
}

// Styles for tree rendering
var (
	treeIndent      = "  "
	treeBranch      = "├─"
	treeLastBranch  = "└─"
	treeVertical    = "│ "
	treeEmpty       = "  "

	statusOpenStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	statusProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E22E"))
	statusBlockedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F92672"))
	statusClosedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	beadIDStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D4FF"))
	beadTitleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#EEEEEE"))
)

// DefaultTreeOpts returns default tree rendering options
func DefaultTreeOpts() TreeOpts {
	return TreeOpts{
		ShowStatus:   true,
		ShowPriority: false,
		ColorEnabled: true,
		MaxDepth:     10,
	}
}

// RenderDependencyTree renders a dependency tree as ASCII art
func RenderDependencyTree(tree *storage.DependencyTree, opts TreeOpts) string {
	var sb strings.Builder

	// Render root bead
	sb.WriteString(renderBead(tree.Bead, "", true, opts))
	sb.WriteString("\n")

	// Render blocked-by section
	if len(tree.BlockedBy) > 0 {
		sb.WriteString(renderSection("Blocked by:", tree.BlockedBy, treeIndent, opts, 1))
	}

	// Render blocking section
	if len(tree.Blocking) > 0 {
		sb.WriteString(renderSection("Blocks:", tree.Blocking, treeIndent, opts, 1))
	}

	return sb.String()
}

// renderSection renders a section of the dependency tree
func renderSection(title string, trees []*storage.DependencyTree, indent string, opts TreeOpts, depth int) string {
	if depth > opts.MaxDepth {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(indent)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(title))
	sb.WriteString("\n")

	for i, tree := range trees {
		isLast := i == len(trees)-1
		renderTree(&sb, tree, indent, isLast, opts, depth)
	}

	return sb.String()
}

// renderTree recursively renders a dependency tree node
func renderTree(sb *strings.Builder, tree *storage.DependencyTree, indent string, isLast bool, opts TreeOpts, depth int) {
	if depth > opts.MaxDepth {
		return
	}

	// Render the branch character
	sb.WriteString(indent)
	if isLast {
		sb.WriteString(treeLastBranch)
	} else {
		sb.WriteString(treeBranch)
	}
	sb.WriteString(" ")

	// Render the bead
	sb.WriteString(renderBead(tree.Bead, "", false, opts))
	sb.WriteString("\n")

	// Calculate new indent for children
	var newIndent string
	if isLast {
		newIndent = indent + treeEmpty
	} else {
		newIndent = indent + treeVertical
	}

	// Render children (blocked-by and blocking)
	children := append(tree.BlockedBy, tree.Blocking...)
	for i, child := range children {
		childIsLast := i == len(children)-1
		renderTree(sb, child, newIndent, childIsLast, opts, depth+1)
	}
}

// renderBead renders a single bead with optional status and priority
func renderBead(bead *models.Bead, prefix string, isRoot bool, opts TreeOpts) string {
	var parts []string

	// Add ID
	if opts.ColorEnabled {
		parts = append(parts, beadIDStyle.Render(bead.ID))
	} else {
		parts = append(parts, bead.ID)
	}

	// Add title (truncated if too long)
	title := bead.Title
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	if opts.ColorEnabled {
		parts = append(parts, beadTitleStyle.Render(title))
	} else {
		parts = append(parts, title)
	}

	// Add status
	if opts.ShowStatus {
		statusStr := fmt.Sprintf("(%s)", bead.Status)
		if opts.ColorEnabled {
			statusStr = styleStatus(string(bead.Status), statusStr)
		}
		parts = append(parts, statusStr)
	}

	// Add priority
	if opts.ShowPriority {
		parts = append(parts, fmt.Sprintf("[P%d]", bead.Priority))
	}

	return prefix + strings.Join(parts, " ")
}

// styleStatus applies color styling to status strings
func styleStatus(status, text string) string {
	switch models.BeadStatus(status) {
	case models.BeadStatusOpen:
		return statusOpenStyle.Render(text)
	case models.BeadStatusInProgress:
		return statusProgressStyle.Render(text)
	case models.BeadStatusBlocked:
		return statusBlockedStyle.Render(text)
	case models.BeadStatusClosed:
		return statusClosedStyle.Render(text)
	default:
		return text
	}
}

// RenderSimpleDeps renders a simple list of dependencies
func RenderSimpleDeps(bead *models.Bead, blockedBy, blocking []*models.Bead, opts TreeOpts) string {
	var sb strings.Builder

	// Header with root bead
	sb.WriteString(renderBead(bead, "", true, opts))
	sb.WriteString("\n")

	// Blocked by section
	if len(blockedBy) > 0 {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("  Blocked by:"))
		sb.WriteString("\n")
		for _, b := range blockedBy {
			sb.WriteString("    ")
			sb.WriteString(renderBead(b, "", false, opts))
			sb.WriteString("\n")
		}
	}

	// Blocks section
	if len(blocking) > 0 {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("  Blocks:"))
		sb.WriteString("\n")
		for _, b := range blocking {
			sb.WriteString("    ")
			sb.WriteString(renderBead(b, "", false, opts))
			sb.WriteString("\n")
		}
	}

	if len(blockedBy) == 0 && len(blocking) == 0 {
		sb.WriteString("  ")
		sb.WriteString(statusOpenStyle.Render("No dependencies"))
		sb.WriteString("\n")
	}

	return sb.String()
}
