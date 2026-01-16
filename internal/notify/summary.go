package notify

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// SummaryReporter collects notifications and generates periodic summary reports
type SummaryReporter struct {
	mu           sync.Mutex
	notifications []Notification
	outputPath   string
	interval     time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewSummaryReporter creates a new summary reporter
func NewSummaryReporter(outputPath string, interval time.Duration) *SummaryReporter {
	return &SummaryReporter{
		notifications: make([]Notification, 0),
		outputPath:   outputPath,
		interval:     interval,
		stopChan:     make(chan struct{}),
	}
}

// Notify adds a notification to the summary
func (s *SummaryReporter) Notify(notification Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = append(s.notifications, notification)
	return nil
}

// Start begins the periodic summary generation
func (s *SummaryReporter) Start() {
	s.wg.Add(1)
	go s.run()
}

// run is the background goroutine that generates summaries
func (s *SummaryReporter) run() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.generateSummary()
		case <-s.stopChan:
			// Generate final summary before exiting
			s.generateSummary()
			return
		}
	}
}

// generateSummary creates a summary report of recent notifications
func (s *SummaryReporter) generateSummary() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.notifications) == 0 {
		return nil
	}

	// Count notifications by type
	typeCounts := make(map[NotificationType]int)
	for _, n := range s.notifications {
		typeCounts[n.Type]++
	}

	// Generate summary
	var output io.Writer
	if s.outputPath != "" {
		f, err := os.OpenFile(s.outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open summary file: %w", err)
		}
		defer f.Close()
		output = f
	} else {
		output = os.Stdout
	}

	fmt.Fprintf(output, "\n=== Notification Summary (%s) ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(output, "Total notifications: %d\n", len(s.notifications))
	for typ, count := range typeCounts {
		fmt.Fprintf(output, "  %s: %d\n", typ, count)
	}

	// List recent notifications (last 10)
	fmt.Fprintf(output, "\nRecent notifications:\n")
	start := len(s.notifications) - 10
	if start < 0 {
		start = 0
	}
	for i := start; i < len(s.notifications); i++ {
		n := s.notifications[i]
		fmt.Fprintf(output, "  [%s] %s: %s\n", n.Timestamp.Format("15:04:05"), n.Type, n.Title)
	}
	fmt.Fprintf(output, "\n")

	// Clear notifications after summary
	s.notifications = make([]Notification, 0)

	return nil
}

// Close stops the summary reporter
func (s *SummaryReporter) Close() error {
	close(s.stopChan)
	s.wg.Wait()
	return nil
}
