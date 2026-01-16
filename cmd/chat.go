package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/underboss"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session with the Underboss",
	Long:  `Launch an interactive conversation with the Underboss to discuss tasks, ask questions, and assign work.`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Get mob directory
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting mob directory: %v\n", err)
			os.Exit(1)
		}

		// 2. Create spawner
		spawner := agent.NewSpawner()

		// 3. Create and start Underboss
		ub := underboss.New(mobDir, spawner)

		// Set up context with cancellation for Ctrl+C
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle interrupt signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nReceived interrupt signal, shutting down...")
			cancel()
		}()

		// Start the Underboss
		if err := ub.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting Underboss: %v\n", err)
			os.Exit(1)
		}

		// Ensure cleanup on exit
		defer func() {
			if err := ub.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error stopping Underboss: %v\n", err)
			}
		}()

		// 4. Create session with os.Stdin/os.Stdout
		session := underboss.NewSession(ub, os.Stdin, os.Stdout)

		// 5. Run session
		if err := session.Run(ctx); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "Error during session: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)
}
