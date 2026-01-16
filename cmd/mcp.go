package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/mcp"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/storage"
	"github.com/spf13/cobra"
)

var (
	mcpRegistryPath string
	mcpMobDir       string
)

var mcpServerCmd = &cobra.Command{
	Use:    "mcp-server",
	Short:  "Run the MCP server for agent management tools",
	Long:   `Runs an MCP server over stdio that exposes tools for spawning and managing Soldati and Associates.`,
	Hidden: true, // Hidden because it's invoked by Claude, not humans
	Run: func(cmd *cobra.Command, args []string) {
		// Determine mob directory
		mobDir := mcpMobDir
		if mobDir == "" {
			var err error
			mobDir, err = getMobDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting mob directory: %v\n", err)
				os.Exit(1)
			}
		}

		// Determine registry path
		registryPath := mcpRegistryPath
		if registryPath == "" {
			registryPath = registry.DefaultPath(mobDir)
		}

		// Create registry and spawner
		reg := registry.New(registryPath)
		spawner := agent.NewSpawner()

		// Create bead store
		beadDir := filepath.Join(mobDir, ".mob", "beads")
		beadStore, err := storage.NewBeadStore(beadDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating bead store: %v\n", err)
			os.Exit(1)
		}

		// Create and run MCP server
		server := mcp.NewServer(reg, spawner, beadStore, mobDir)
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	mcpServerCmd.Flags().StringVar(&mcpRegistryPath, "registry", "", "Path to agent registry file")
	mcpServerCmd.Flags().StringVar(&mcpMobDir, "mob-dir", "", "Mob directory path")
	rootCmd.AddCommand(mcpServerCmd)
}
