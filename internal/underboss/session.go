package underboss

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Session handles an interactive chat session with the Underboss
type Session struct {
	underboss *Underboss
	input     io.Reader
	output    io.Writer
}

// NewSession creates a new chat session
func NewSession(underboss *Underboss, input io.Reader, output io.Writer) *Session {
	return &Session{
		underboss: underboss,
		input:     input,
		output:    output,
	}
}

// Run starts the interactive session, returns when user exits
func (s *Session) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.input)

	s.printWelcome()

	for {
		select {
		case <-ctx.Done():
			s.printGoodbye()
			return ctx.Err()
		default:
		}

		fmt.Fprint(s.output, "\n> ")

		if !scanner.Scan() {
			// EOF or error
			if err := scanner.Err(); err != nil {
				return err
			}
			s.printGoodbye()
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Check for exit commands
		if s.isExitCommand(input) {
			s.printGoodbye()
			return nil
		}

		// Send message to Underboss and get response
		if err := s.sendMessage(ctx, input); err != nil {
			fmt.Fprintf(s.output, "Error: %v\n", err)
			continue
		}
	}
}

// isExitCommand checks if the input is an exit command
func (s *Session) isExitCommand(input string) bool {
	lower := strings.ToLower(input)
	return lower == "exit" || lower == "quit" || lower == "q"
}

// sendMessage sends a message to the Underboss and displays the response
func (s *Session) sendMessage(ctx context.Context, message string) error {
	if !s.underboss.IsRunning() {
		return ErrUnderbossNotRunning
	}

	agent := s.underboss.Agent()
	if agent == nil {
		return ErrUnderbossNotRunning
	}

	// Send the message using the chat method
	params := map[string]interface{}{
		"message": message,
	}

	resp, err := agent.Call("chat", params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("underboss error: %s", resp.Error.Message)
	}

	// Parse and display the response
	if resp.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			// Just print raw result if we can't parse it
			fmt.Fprintf(s.output, "\n%s\n", string(resp.Result))
			return nil
		}

		if response, ok := result["response"].(string); ok {
			fmt.Fprintf(s.output, "\n%s\n", response)
		} else {
			// Print the whole result if we can't find a response field
			formatted, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintf(s.output, "\n%s\n", string(formatted))
		}
	}

	return nil
}

// printWelcome displays the welcome message
func (s *Session) printWelcome() {
	fmt.Fprintln(s.output, "")
	fmt.Fprintln(s.output, "==============================================")
	fmt.Fprintln(s.output, "  Welcome to the Mob - Chat with the Underboss")
	fmt.Fprintln(s.output, "==============================================")
	fmt.Fprintln(s.output, "")
	fmt.Fprintln(s.output, "Type your message and press Enter to send.")
	fmt.Fprintln(s.output, "Type 'exit', 'quit', or 'q' to leave.")
	fmt.Fprintln(s.output, "Press Ctrl+C to exit immediately.")
}

// printGoodbye displays the goodbye message
func (s *Session) printGoodbye() {
	fmt.Fprintln(s.output, "")
	fmt.Fprintln(s.output, "The Underboss says goodbye.")
	fmt.Fprintln(s.output, "")
}
