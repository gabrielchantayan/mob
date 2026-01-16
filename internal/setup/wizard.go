package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabe/mob/internal/config"
)

// Wizard handles interactive first-run setup
type Wizard struct {
	reader *bufio.Reader
}

// NewWizard creates a setup wizard
func NewWizard() *Wizard {
	return &Wizard{
		reader: bufio.NewReader(os.Stdin),
	}
}

// Run executes the setup wizard
func (w *Wizard) Run() error {
	fmt.Println("Welcome to Mob - Claude Code Agent Orchestrator")
	fmt.Println("================================================")
	fmt.Println()

	// Get mob home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultMobDir := filepath.Join(homeDir, "mob")
	mobDir, err := w.prompt("Where should mob store its data?", defaultMobDir)
	if err != nil {
		return err
	}

	// Create directory structure
	dirs := []string{
		mobDir,
		filepath.Join(mobDir, ".mob"),
		filepath.Join(mobDir, ".mob", "logs"),
		filepath.Join(mobDir, ".mob", "logs", "soldati"),
		filepath.Join(mobDir, ".mob", "tmp"),
		filepath.Join(mobDir, ".mob", "soldati"),
		filepath.Join(mobDir, "beads"),
		filepath.Join(mobDir, "beads", "archive"),
		filepath.Join(mobDir, "soldati"),
		filepath.Join(mobDir, "history"),
		filepath.Join(mobDir, "history", "summaries"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default config
	configPath := filepath.Join(mobDir, "config.toml")
	cfg := config.DefaultConfig()
	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create empty turfs file
	turfsPath := filepath.Join(mobDir, "turfs.toml")
	if err := os.WriteFile(turfsPath, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create turfs file: %w", err)
	}

	fmt.Println()
	fmt.Println("Setup complete!")
	fmt.Printf("  Mob home: %s\n", mobDir)
	fmt.Printf("  Config:   %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Register a project: mob turf add /path/to/project")
	fmt.Println("  2. Start the daemon:   mob daemon start")
	fmt.Println("  3. Chat with mob:      mob chat")

	return nil
}

func (w *Wizard) prompt(question, defaultVal string) (string, error) {
	fmt.Printf("%s [%s]: ", question, defaultVal)
	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}
