package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/gabe/mob/internal/heresy"
	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/turf"
	"github.com/spf13/cobra"
)

var heresyCmd = &cobra.Command{
	Use:   "heresy",
	Short: "Detect and eradicate architectural anti-patterns",
	Long: `Detect and eradicate architectural anti-patterns (heresies) in your codebase.

Heresies are wrong architectural assumptions that spread through the codebase,
creating inconsistencies and technical debt. This command helps you find and
fix them systematically.

Available subcommands:
  scan  - Scan for new heresies
  list  - List known heresies from beads
  purge - Create fix beads for each location of a heresy`,
}

var heresyScanCmd = &cobra.Command{
	Use:   "scan [turf]",
	Short: "Scan for heresies in the codebase",
	Long: `Scan a turf for architectural anti-patterns.

This command detects various types of heresies:
  - Naming inconsistencies (camelCase vs snake_case)
  - Deprecated pattern usage
  - Copy-paste code that diverged
  - Import alias inconsistencies

Each detected heresy can be converted to a bead for tracking and remediation.

If no turf is specified, uses the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runHeresyScan,
}

var heresyListCmd = &cobra.Command{
	Use:   "list [turf]",
	Short: "List known heresies",
	Long: `List known heresies from existing heresy beads.

This shows all beads of type "heresy" that have been created,
either from previous scans or manually.

If no turf is specified, lists all heresies.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runHeresyList,
}

var heresyPurgeCmd = &cobra.Command{
	Use:   "purge <bead-id>",
	Short: "Eradicate a heresy by creating fix beads",
	Long: `Eradicate a heresy by creating child beads for each affected location.

Given a heresy bead ID, this command creates a child bead for each
location where the heresy appears. Each child bead tracks the fix
for that specific location, making the remediation process tractable.

The parent heresy bead remains open until all child beads are resolved.`,
	Args: cobra.ExactArgs(1),
	Run:  runHeresyPurge,
}

// Flags
var (
	heresyCreateBeads bool
)

func init() {
	heresyScanCmd.Flags().BoolVar(&heresyCreateBeads, "create-beads", false, "Create beads for detected heresies")

	heresyCmd.AddCommand(heresyScanCmd)
	heresyCmd.AddCommand(heresyListCmd)
	heresyCmd.AddCommand(heresyPurgeCmd)
	rootCmd.AddCommand(heresyCmd)
}

func runHeresyScan(cmd *cobra.Command, args []string) {
	turfPath, err := resolveHeresyTurfPath(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	detector, err := createDetector(turfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanning for heresies in %s...\n\n", turfPath)

	ctx := context.Background()
	heresies, err := detector.Scan(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning for heresies: %v\n", err)
		os.Exit(1)
	}

	if len(heresies) == 0 {
		fmt.Println("No heresies detected. The codebase is pure.")
		return
	}

	// Print detected heresies
	fmt.Printf("Found %d heresies:\n\n", len(heresies))
	printHeresies(heresies)

	// Create beads if requested
	if heresyCreateBeads {
		fmt.Println("\nCreating beads for heresies...")
		beadIDs, err := detector.CreateBeads(heresies)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating beads: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created %d heresy beads:\n", len(beadIDs))
		for _, id := range beadIDs {
			fmt.Printf("  %s\n", id)
		}
	} else {
		fmt.Println("\nUse --create-beads to create beads for these heresies.")
	}
}

func runHeresyList(cmd *cobra.Command, args []string) {
	turfPath := ""
	if len(args) > 0 {
		var err error
		turfPath, err = resolveHeresyTurfPath(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Get bead store
	beadDir, err := getBeadStorePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating bead store: %v\n", err)
		os.Exit(1)
	}

	detector := heresy.New(turfPath, beadStore)

	ctx := context.Background()
	heresies, err := detector.List(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing heresies: %v\n", err)
		os.Exit(1)
	}

	if len(heresies) == 0 {
		fmt.Println("No known heresies. The faith is strong.")
		return
	}

	fmt.Printf("Known heresies (%d):\n\n", len(heresies))
	printHeresies(heresies)
}

func runHeresyPurge(cmd *cobra.Command, args []string) {
	beadID := args[0]

	// Get current directory as turf path
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	detector, err := createDetector(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Purging heresy %s...\n\n", beadID)

	ctx := context.Background()
	childIDs, err := detector.Purge(ctx, beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error purging heresy: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %d fix beads:\n", len(childIDs))
	for _, id := range childIDs {
		fmt.Printf("  %s\n", id)
	}

	fmt.Println("\nEach bead represents one location to fix. Work through them systematically.")
}

// resolveHeresyTurfPath resolves the turf path from arguments or current directory
func resolveHeresyTurfPath(args []string) (string, error) {
	if len(args) > 0 {
		turfNameOrPath := args[0]

		// First, try to resolve as a registered turf name
		turfsPath, err := getTurfsPath()
		if err == nil {
			mgr, err := turf.NewManager(turfsPath)
			if err == nil {
				t, err := mgr.Get(turfNameOrPath)
				if err == nil {
					return t.Path, nil
				}
			}
		}

		// Otherwise, treat it as a path
		absPath, err := filepath.Abs(turfNameOrPath)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		// Verify the path exists
		if _, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}

		return absPath, nil
	}

	// Use current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return cwd, nil
}

// createDetector creates a new heresy Detector for the given turf path
func createDetector(turfPath string) (*heresy.Detector, error) {
	// Get bead store directory
	beadDir, err := getBeadStorePath()
	if err != nil {
		return nil, err
	}

	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bead store: %w", err)
	}

	return heresy.New(turfPath, beadStore), nil
}

// printHeresies prints heresies in a formatted table
func printHeresies(heresies []*heresy.Heresy) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for i, h := range heresies {
		fmt.Fprintf(w, "--- Heresy #%d ---\n", i+1)
		fmt.Fprintf(w, "ID:\t%s\n", h.ID)
		fmt.Fprintf(w, "Description:\t%s\n", h.Description)
		fmt.Fprintf(w, "Pattern:\t%s\n", h.Pattern)
		if h.Correct != "" {
			fmt.Fprintf(w, "Correct:\t%s\n", h.Correct)
		}
		fmt.Fprintf(w, "Severity:\t%s\n", h.Severity)
		fmt.Fprintf(w, "Spread:\t%d locations\n", h.Spread)
		if len(h.Locations) > 0 {
			fmt.Fprintf(w, "Locations:\n")
			limit := len(h.Locations)
			if limit > 5 {
				limit = 5
			}
			for _, loc := range h.Locations[:limit] {
				fmt.Fprintf(w, "  - %s\n", loc)
			}
			if len(h.Locations) > 5 {
				fmt.Fprintf(w, "  ... and %d more\n", len(h.Locations)-5)
			}
		}
		fmt.Fprintln(w)
	}

	w.Flush()
}
