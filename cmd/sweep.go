package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gabe/mob/internal/storage"
	"github.com/gabe/mob/internal/sweep"
	"github.com/gabe/mob/internal/turf"
	"github.com/spf13/cobra"
)

var sweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Run maintenance sweeps across a turf",
	Long: `Run maintenance sweeps to find issues in your codebase.

Sweeps analyze your code for potential problems and create beads
(work items) for each issue found.

Available sweep types:
  review - Code review sweep (style issues, missing tests, security)
  bugs   - Bugfix sweep (TODO/FIXME/HACK comments, error handling)
  all    - Run all sweep types`,
}

var sweepReviewCmd = &cobra.Command{
	Use:   "review [turf]",
	Short: "Run a code review sweep",
	Long: `Run a code review sweep on a turf.

This sweep looks for:
  - Recent commits with WIP/TODO markers
  - Debug print statements left in code
  - Potential security issues
  - Style inconsistencies

If no turf is specified, uses the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSweepReview,
}

var sweepBugsCmd = &cobra.Command{
	Use:   "bugs [turf]",
	Short: "Run a bugfix sweep",
	Long: `Run a bugfix sweep on a turf.

This sweep hunts for:
  - TODO comments
  - FIXME markers
  - HACK workarounds
  - XXX and BUG annotations

Each found item becomes a bead that can be tracked and worked on.

If no turf is specified, uses the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSweepBugs,
}

var sweepAllCmd = &cobra.Command{
	Use:   "all [turf]",
	Short: "Run all sweeps",
	Long: `Run all available sweeps on a turf.

This runs both the review and bugs sweeps, combining their results.

If no turf is specified, uses the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSweepAll,
}

func init() {
	sweepCmd.AddCommand(sweepReviewCmd)
	sweepCmd.AddCommand(sweepBugsCmd)
	sweepCmd.AddCommand(sweepAllCmd)
	rootCmd.AddCommand(sweepCmd)
}

func runSweepReview(cmd *cobra.Command, args []string) {
	turfPath, err := resolveTurfPath(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sweeper, err := createSweeper(turfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running code review sweep on %s...\n\n", turfPath)

	ctx := context.Background()
	result, err := sweeper.Review(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running review sweep: %v\n", err)
		os.Exit(1)
	}

	printSweepResult(result)
}

func runSweepBugs(cmd *cobra.Command, args []string) {
	turfPath, err := resolveTurfPath(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sweeper, err := createSweeper(turfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running bug sweep on %s...\n\n", turfPath)

	ctx := context.Background()
	result, err := sweeper.Bugs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running bug sweep: %v\n", err)
		os.Exit(1)
	}

	printSweepResult(result)
}

func runSweepAll(cmd *cobra.Command, args []string) {
	turfPath, err := resolveTurfPath(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sweeper, err := createSweeper(turfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running all sweeps on %s...\n\n", turfPath)

	ctx := context.Background()
	results, err := sweeper.All(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running sweeps: %v\n", err)
		os.Exit(1)
	}

	for i, result := range results {
		printSweepResult(result)
		if i < len(results)-1 {
			fmt.Println()
		}
	}

	// Print summary
	totalItems := 0
	totalBeads := 0
	for _, r := range results {
		totalItems += r.ItemsFound
		totalBeads += len(r.Beads)
	}
	fmt.Printf("\nAll sweeps complete: %d issues found, %d beads created\n", totalItems, totalBeads)
}

// resolveTurfPath resolves the turf path from arguments or current directory
func resolveTurfPath(args []string) (string, error) {
	if len(args) > 0 {
		// Check if it's a turf name or a path
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

// createSweeper creates a new Sweeper for the given turf path
func createSweeper(turfPath string) (*sweep.Sweeper, error) {
	// Get bead store directory
	beadDir, err := getBeadStorePath()
	if err != nil {
		return nil, err
	}

	beadStore, err := storage.NewBeadStore(beadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bead store: %w", err)
	}

	return sweep.New(turfPath, beadStore), nil
}

// getBeadStorePath returns the path to the bead store
func getBeadStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, "mob", ".mob", "beads"), nil
}

// printSweepResult prints a sweep result to stdout
func printSweepResult(result *sweep.SweepResult) {
	fmt.Printf("=== %s Sweep ===\n", string(result.Type))
	fmt.Printf("Turf: %s\n", result.Turf)
	fmt.Printf("Duration: %v\n", result.CompletedAt.Sub(result.StartedAt).Round(time.Millisecond))
	fmt.Printf("Items Found: %d\n", result.ItemsFound)
	fmt.Printf("Beads Created: %d\n", len(result.Beads))

	if len(result.Beads) > 0 {
		fmt.Println("\nCreated Beads:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, beadID := range result.Beads {
			fmt.Fprintf(w, "  %s\n", beadID)
		}
		w.Flush()
	}

	fmt.Printf("\n%s\n", result.Summary)
}
