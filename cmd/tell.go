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

var tellCmd = &cobra.Command{
	Use:   "tell <instruction>",
	Short: "Give the Underboss an instruction",
	Long:  `Send a one-shot instruction to the Underboss for execution.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		instruction := args[0]

		// 1. Get mob directory
		mobDir, err := getMobDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting mob directory: %v\n", err)
			os.Exit(1)
		}

		// 2. Create spawner
		spawner := agent.NewSpawner()

		// 3. Create Underboss
		ub := underboss.New(mobDir, spawner)

		// Set up context with cancellation for Ctrl+C
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle interrupt signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down...")
			cancel()
		}()

		// Ensure cleanup on exit
		defer func() {
			if ub.IsRunning() {
				if err := ub.Stop(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: error stopping Underboss: %v\n", err)
				}
			}
		}()

		// 4. Call Tell(ctx, instruction)
		acknowledgment, err := ub.Tell(ctx, instruction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 5. Print acknowledgment
		fmt.Println(acknowledgment)
	},
}

func init() {
	rootCmd.AddCommand(tellCmd)
}
