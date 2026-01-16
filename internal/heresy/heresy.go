// Package heresy provides detection and eradication of architectural anti-patterns.
// Heresies are wrong architectural assumptions that spread through the codebase,
// creating inconsistencies and technical debt that compounds over time.
package heresy

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gabe/mob/internal/models"
	"github.com/gabe/mob/internal/storage"
)

// Severity levels for heresies
type Severity string

const (
	// SeverityLow indicates a minor style or consistency issue
	SeverityLow Severity = "low"
	// SeverityMedium indicates a notable anti-pattern
	SeverityMedium Severity = "medium"
	// SeverityHigh indicates a significant architectural issue
	SeverityHigh Severity = "high"
	// SeverityCritical indicates a severe anti-pattern requiring immediate attention
	SeverityCritical Severity = "critical"
)

// Heresy represents a detected architectural anti-pattern
type Heresy struct {
	ID          string    // Unique identifier
	Description string    // What's wrong
	Pattern     string    // What to look for
	Correct     string    // The correct pattern (if known)
	Locations   []string  // Files where it appears
	Spread      int       // Number of occurrences
	Severity    Severity  // "low", "medium", "high", "critical"
	DetectedAt  time.Time // When detected
}

// Detector scans for heresies in a codebase
type Detector struct {
	turfPath  string
	beadStore *storage.BeadStore
}

// New creates a new Detector for a given turf
func New(turfPath string, beadStore *storage.BeadStore) *Detector {
	return &Detector{
		turfPath:  turfPath,
		beadStore: beadStore,
	}
}

// Scan scans the codebase for heresies
func (d *Detector) Scan(ctx context.Context) ([]*Heresy, error) {
	heresies := make([]*Heresy, 0)

	// Check for pattern inconsistencies (naming conventions)
	namingHeresies, err := d.detectNamingInconsistencies(ctx)
	if err == nil {
		heresies = append(heresies, namingHeresies...)
	}

	// Find deprecated patterns still in use
	deprecatedHeresies, err := d.detectDeprecatedUsage(ctx)
	if err == nil {
		heresies = append(heresies, deprecatedHeresies...)
	}

	// Detect copy-paste code that diverged (similar function signatures)
	copyPasteHeresies, err := d.detectCopyPasteCode(ctx)
	if err == nil {
		heresies = append(heresies, copyPasteHeresies...)
	}

	// Find import inconsistencies
	importHeresies, err := d.detectImportInconsistencies(ctx)
	if err == nil {
		heresies = append(heresies, importHeresies...)
	}

	return heresies, nil
}

// List returns known heresies (from beads with type "heresy")
func (d *Detector) List(ctx context.Context) ([]*Heresy, error) {
	beads, err := d.beadStore.List(storage.BeadFilter{
		Type: models.BeadTypeHeresy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list heresy beads: %w", err)
	}

	var heresies []*Heresy
	for _, bead := range beads {
		heresy := d.beadToHeresy(bead)
		heresies = append(heresies, heresy)
	}

	return heresies, nil
}

// CreateBeads creates beads for detected heresies
func (d *Detector) CreateBeads(heresies []*Heresy) ([]string, error) {
	var beadIDs []string

	for _, h := range heresies {
		bead := d.heresyToBead(h)
		created, err := d.beadStore.Create(bead)
		if err != nil {
			return beadIDs, fmt.Errorf("failed to create bead for heresy %s: %w", h.ID, err)
		}
		beadIDs = append(beadIDs, created.ID)
	}

	return beadIDs, nil
}

// Purge creates child beads for each location of a heresy
func (d *Detector) Purge(ctx context.Context, heresyBeadID string) ([]string, error) {
	// Get the heresy bead
	parentBead, err := d.beadStore.Get(heresyBeadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get heresy bead: %w", err)
	}

	// Extract locations from the bead's labels (comma-separated)
	locations := d.extractLocations(parentBead)
	if len(locations) == 0 {
		return nil, fmt.Errorf("no locations found in heresy bead")
	}

	var childIDs []string
	for _, location := range locations {
		childBead := &models.Bead{
			Title:       fmt.Sprintf("Fix heresy at %s", location),
			Description: fmt.Sprintf("Fix the heresy from parent bead %s at location: %s\n\nOriginal issue: %s", parentBead.ID, location, parentBead.Title),
			Status:      models.BeadStatusOpen,
			Type:        models.BeadTypeChore,
			Turf:        d.turfPath,
			Priority:    parentBead.Priority,
			ParentID:    parentBead.ID,
		}

		created, err := d.beadStore.Create(childBead)
		if err != nil {
			return childIDs, fmt.Errorf("failed to create child bead for location %s: %w", location, err)
		}
		childIDs = append(childIDs, created.ID)
	}

	return childIDs, nil
}

// detectNamingInconsistencies finds mixed naming conventions (camelCase vs snake_case)
func (d *Detector) detectNamingInconsistencies(ctx context.Context) ([]*Heresy, error) {
	var heresies []*Heresy

	// Patterns for detecting naming style inconsistencies
	snakeCaseFunc := regexp.MustCompile(`func\s+([a-z]+_[a-z_]+)\s*\(`)
	camelCaseFunc := regexp.MustCompile(`func\s+([a-z][a-zA-Z0-9]*[A-Z][a-zA-Z0-9]*)\s*\(`)

	snakeLocations := make(map[string][]string) // pattern -> locations
	camelLocations := make(map[string][]string)

	err := filepath.Walk(d.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		relPath, _ := filepath.Rel(d.turfPath, path)

		for lineNum, line := range lines {
			if matches := snakeCaseFunc.FindStringSubmatch(line); len(matches) > 1 {
				location := fmt.Sprintf("%s:%d", relPath, lineNum+1)
				snakeLocations[matches[1]] = append(snakeLocations[matches[1]], location)
			}
			if matches := camelCaseFunc.FindStringSubmatch(line); len(matches) > 1 {
				location := fmt.Sprintf("%s:%d", relPath, lineNum+1)
				camelLocations[matches[1]] = append(camelLocations[matches[1]], location)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// If we have both snake_case and camelCase functions, it's a naming heresy
	if len(snakeLocations) > 0 && len(camelLocations) > 0 {
		var allSnakeLocations []string
		for _, locs := range snakeLocations {
			allSnakeLocations = append(allSnakeLocations, locs...)
		}

		heresy := &Heresy{
			ID:          generateHeresyID(),
			Description: "Inconsistent naming convention: mixing snake_case with camelCase",
			Pattern:     "func snake_case_name()",
			Correct:     "Use consistent camelCase for Go functions",
			Locations:   allSnakeLocations,
			Spread:      len(allSnakeLocations),
			Severity:    SeverityMedium,
			DetectedAt:  time.Now(),
		}
		heresies = append(heresies, heresy)
	}

	return heresies, nil
}

// detectDeprecatedUsage finds usage of deprecated functions/patterns
func (d *Detector) detectDeprecatedUsage(ctx context.Context) ([]*Heresy, error) {
	var heresies []*Heresy

	// First pass: find deprecated markers
	deprecatedPattern := regexp.MustCompile(`(?i)//\s*deprecated:?\s*(.*)`)
	deprecated := make(map[string]string) // function name -> replacement suggestion

	err := filepath.Walk(d.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !isCodeFile(ext) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var lastDeprecation string
		for scanner.Scan() {
			line := scanner.Text()

			// Check for deprecation comment
			if matches := deprecatedPattern.FindStringSubmatch(line); len(matches) > 1 {
				lastDeprecation = strings.TrimSpace(matches[1])
				continue
			}

			// If previous line was deprecation, this might be the function
			if lastDeprecation != "" {
				funcPattern := regexp.MustCompile(`func\s+(\w+)\s*\(`)
				if matches := funcPattern.FindStringSubmatch(line); len(matches) > 1 {
					deprecated[matches[1]] = lastDeprecation
				}
				lastDeprecation = ""
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Second pass: find usage of deprecated functions
	for funcName, replacement := range deprecated {
		usagePattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*\(`, regexp.QuoteMeta(funcName)))
		var locations []string

		err := filepath.Walk(d.turfPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := filepath.Ext(path)
			if !isCodeFile(ext) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			lines := strings.Split(string(content), "\n")
			relPath, _ := filepath.Rel(d.turfPath, path)

			for lineNum, line := range lines {
				// Skip the line where it's defined
				if strings.Contains(line, "func "+funcName) {
					continue
				}
				if usagePattern.MatchString(line) {
					locations = append(locations, fmt.Sprintf("%s:%d", relPath, lineNum+1))
				}
			}

			return nil
		})

		if err != nil {
			continue
		}

		if len(locations) > 0 {
			heresy := &Heresy{
				ID:          generateHeresyID(),
				Description: fmt.Sprintf("Usage of deprecated function: %s", funcName),
				Pattern:     funcName + "()",
				Correct:     replacement,
				Locations:   locations,
				Spread:      len(locations),
				Severity:    SeverityHigh,
				DetectedAt:  time.Now(),
			}
			heresies = append(heresies, heresy)
		}
	}

	return heresies, nil
}

// detectCopyPasteCode finds similar code patterns that may have diverged
func (d *Detector) detectCopyPasteCode(ctx context.Context) ([]*Heresy, error) {
	var heresies []*Heresy

	// Look for similar function signatures that might indicate copy-paste
	funcSignatures := make(map[string][]string) // normalized signature -> locations

	funcPattern := regexp.MustCompile(`func\s+(\w+)\s*\(([^)]*)\)\s*([^{]*)`)

	err := filepath.Walk(d.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		relPath, _ := filepath.Rel(d.turfPath, path)

		for lineNum, line := range lines {
			if matches := funcPattern.FindStringSubmatch(line); len(matches) > 3 {
				// Normalize: remove spaces and variable names, keep types
				params := normalizeParams(matches[2])
				returns := strings.TrimSpace(matches[3])
				signature := fmt.Sprintf("(%s)%s", params, returns)

				location := fmt.Sprintf("%s:%d:%s", relPath, lineNum+1, matches[1])
				funcSignatures[signature] = append(funcSignatures[signature], location)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find signatures that appear multiple times (potential copy-paste)
	for signature, locations := range funcSignatures {
		if len(locations) >= 3 { // At least 3 similar functions suggests copy-paste
			heresy := &Heresy{
				ID:          generateHeresyID(),
				Description: "Potential copy-paste code: multiple functions with identical signatures",
				Pattern:     signature,
				Correct:     "Consider extracting shared logic or using generics",
				Locations:   locations,
				Spread:      len(locations),
				Severity:    SeverityLow,
				DetectedAt:  time.Now(),
			}
			heresies = append(heresies, heresy)
		}
	}

	return heresies, nil
}

// detectImportInconsistencies finds inconsistent import aliasing
func (d *Detector) detectImportInconsistencies(ctx context.Context) ([]*Heresy, error) {
	var heresies []*Heresy

	// Track import aliases per package
	importAliases := make(map[string]map[string][]string) // package -> alias -> locations

	importPattern := regexp.MustCompile(`(\w+)?\s*"([^"]+)"`)

	err := filepath.Walk(d.turfPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Find import block
		inImport := false
		lines := strings.Split(string(content), "\n")
		relPath, _ := filepath.Rel(d.turfPath, path)

		for lineNum, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "import (") {
				inImport = true
				continue
			}
			if inImport && trimmed == ")" {
				inImport = false
				continue
			}
			if inImport || strings.HasPrefix(trimmed, "import ") {
				if matches := importPattern.FindStringSubmatch(line); len(matches) > 2 {
					alias := matches[1]
					pkg := matches[2]

					if importAliases[pkg] == nil {
						importAliases[pkg] = make(map[string][]string)
					}
					location := fmt.Sprintf("%s:%d", relPath, lineNum+1)
					importAliases[pkg][alias] = append(importAliases[pkg][alias], location)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Find packages with multiple aliases
	for pkg, aliases := range importAliases {
		if len(aliases) > 1 {
			var allLocations []string
			var aliasNames []string
			for alias, locs := range aliases {
				if alias == "" {
					aliasNames = append(aliasNames, "(no alias)")
				} else {
					aliasNames = append(aliasNames, alias)
				}
				allLocations = append(allLocations, locs...)
			}

			heresy := &Heresy{
				ID:          generateHeresyID(),
				Description: fmt.Sprintf("Inconsistent import aliases for package: %s", pkg),
				Pattern:     fmt.Sprintf("aliases: %s", strings.Join(aliasNames, ", ")),
				Correct:     "Use consistent import aliases across the codebase",
				Locations:   allLocations,
				Spread:      len(allLocations),
				Severity:    SeverityLow,
				DetectedAt:  time.Now(),
			}
			heresies = append(heresies, heresy)
		}
	}

	return heresies, nil
}

// beadToHeresy converts a bead to a Heresy struct
func (d *Detector) beadToHeresy(bead *models.Bead) *Heresy {
	locations := d.extractLocations(bead)

	return &Heresy{
		ID:          bead.ID,
		Description: bead.Description,
		Pattern:     bead.Title,
		Locations:   locations,
		Spread:      len(locations),
		Severity:    d.priorityToSeverity(bead.Priority),
		DetectedAt:  bead.CreatedAt,
	}
}

// heresyToBead converts a Heresy to a bead
func (d *Detector) heresyToBead(h *Heresy) *models.Bead {
	description := h.Description
	if h.Correct != "" {
		description += fmt.Sprintf("\n\nCorrect pattern: %s", h.Correct)
	}
	if len(h.Locations) > 0 {
		description += fmt.Sprintf("\n\nLocations:\n- %s", strings.Join(h.Locations, "\n- "))
	}

	return &models.Bead{
		Title:          fmt.Sprintf("[HERESY] %s", h.Pattern),
		Description:    description,
		Status:         models.BeadStatusOpen,
		Type:           models.BeadTypeHeresy,
		Turf:           d.turfPath,
		Priority:       d.severityToPriority(h.Severity),
		Labels:         strings.Join(h.Locations, ","),
		DiscoveredFrom: "heresy-scan",
	}
}

// extractLocations extracts locations from a bead's labels field
func (d *Detector) extractLocations(bead *models.Bead) []string {
	if bead.Labels == "" {
		return nil
	}
	locations := strings.Split(bead.Labels, ",")
	var cleaned []string
	for _, loc := range locations {
		loc = strings.TrimSpace(loc)
		if loc != "" {
			cleaned = append(cleaned, loc)
		}
	}
	return cleaned
}

// severityToPriority converts severity to bead priority
func (d *Detector) severityToPriority(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 2
	}
}

// priorityToSeverity converts bead priority to severity
func (d *Detector) priorityToSeverity(priority int) Severity {
	switch priority {
	case 0:
		return SeverityCritical
	case 1:
		return SeverityHigh
	case 2:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// generateHeresyID creates a unique ID for a heresy
func generateHeresyID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("heresy-%d", time.Now().UnixNano())
	}
	return "heresy-" + hex.EncodeToString(b)
}

// normalizeParams normalizes function parameters for signature comparison
func normalizeParams(params string) string {
	// Remove parameter names, keep only types
	parts := strings.Split(params, ",")
	var types []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Get the last word (the type)
		words := strings.Fields(part)
		if len(words) > 0 {
			types = append(types, words[len(words)-1])
		}
	}
	return strings.Join(types, ",")
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
