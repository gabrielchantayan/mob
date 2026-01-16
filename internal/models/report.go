package models

import "time"

// ReportType represents the type of agent report
type ReportType string

const (
	ReportTypeCompleted  ReportType = "completed"
	ReportTypeBlocked    ReportType = "blocked"
	ReportTypeQuestion   ReportType = "question"
	ReportTypeEscalation ReportType = "escalation"
	ReportTypeProgress   ReportType = "progress"
)

// AgentReport represents a report from an agent to the underboss
type AgentReport struct {
	ID        string     `json:"id"`
	AgentID   string     `json:"agent_id"`
	AgentName string     `json:"agent_name"`
	BeadID    string     `json:"bead_id,omitempty"`
	Type      ReportType `json:"type"`
	Message   string     `json:"message"`
	Timestamp time.Time  `json:"timestamp"`
	Handled   bool       `json:"handled"`
}
