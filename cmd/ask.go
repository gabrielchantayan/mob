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

var askCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Ask the Underboss a question",
	Long:  `Send a one-shot question to the Underboss and receive a response.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		question := args[0]

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

		// 4. Call Ask(ctx, question)
		response, err := ub.Ask(ctx, question)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 5. Print response
		fmt.Println(response)
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
}
