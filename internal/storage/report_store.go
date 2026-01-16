package storage

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gabe/mob/internal/models"
)

// ReportStore manages JSONL-based report storage
type ReportStore struct {
	dir      string
	openFile string
	mu       sync.RWMutex
}

// ReportFilter defines filtering options for listing reports
type ReportFilter struct {
	AgentID   string
	AgentName string
	BeadID    string
	Type      models.ReportType
	Handled   *bool // nil = all, true = handled only, false = unhandled only
}

// NewReportStore creates a new report store at the given directory
func NewReportStore(dir string) (*ReportStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create report directory: %w", err)
	}

	return &ReportStore{
		dir:      dir,
		openFile: filepath.Join(dir, "reports.jsonl"),
	}, nil
}

// generateReportID creates a short random ID for reports
func generateReportID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return "rp-" + hex.EncodeToString(b)[:4], nil
}

// Create adds a new report to the store
func (s *ReportStore) Create(report *models.AgentReport) (*models.AgentReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := generateReportID()
	if err != nil {
		return nil, err
	}
	report.ID = id
	report.Timestamp = time.Now()
	report.Handled = false

	return report, s.appendReport(report)
}

// List returns all reports matching the filter
func (s *ReportStore) List(filter ReportFilter) ([]*models.AgentReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports, err := s.readAllReports()
	if err != nil {
		return nil, err
	}

	// Apply filters
	var filtered []*models.AgentReport
	for _, report := range reports {
		if filter.AgentID != "" && report.AgentID != filter.AgentID {
			continue
		}
		if filter.AgentName != "" && report.AgentName != filter.AgentName {
			continue
		}
		if filter.BeadID != "" && report.BeadID != filter.BeadID {
			continue
		}
		if filter.Type != "" && report.Type != filter.Type {
			continue
		}
		if filter.Handled != nil && report.Handled != *filter.Handled {
			continue
		}
		filtered = append(filtered, report)
	}

	return filtered, nil
}

// Get retrieves a report by ID
func (s *ReportStore) Get(id string) (*models.AgentReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports, err := s.readAllReports()
	if err != nil {
		return nil, err
	}

	for _, report := range reports {
		if report.ID == id {
			return report, nil
		}
	}

	return nil, fmt.Errorf("report not found: %s", id)
}

// MarkHandled marks a report as handled
func (s *ReportStore) MarkHandled(id string) (*models.AgentReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	reports, err := s.readAllReports()
	if err != nil {
		return nil, err
	}

	found := false
	var updatedReport *models.AgentReport
	for i, report := range reports {
		if report.ID == id {
			report.Handled = true
			reports[i] = report
			updatedReport = report
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("report not found: %s", id)
	}

	return updatedReport, s.writeAllReports(reports)
}

func (s *ReportStore) appendReport(report *models.AgentReport) error {
	f, err := os.OpenFile(s.openFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *ReportStore) readAllReports() ([]*models.AgentReport, error) {
	f, err := os.Open(s.openFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reports []*models.AgentReport
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var report models.AgentReport
		if err := json.Unmarshal(scanner.Bytes(), &report); err != nil {
			continue // Skip malformed lines
		}
		reports = append(reports, &report)
	}

	return reports, scanner.Err()
}

func (s *ReportStore) writeAllReports(reports []*models.AgentReport) error {
	// Write to temp file first
	tmpFile := s.openFile + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	for _, report := range reports {
		data, err := json.Marshal(report)
		if err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			f.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return os.Rename(tmpFile, s.openFile)
}
