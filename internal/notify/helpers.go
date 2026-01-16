package notify

import (
	"fmt"
)

// NotifyTaskComplete sends a notification for task completion
func (m *Manager) NotifyTaskComplete(beadID, title, assignee string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeTaskComplete,
		Title:   "Task Completed",
		Message: fmt.Sprintf("%s completed: %s", assignee, title),
		Data: map[string]interface{}{
			"bead_id":  beadID,
			"assignee": assignee,
		},
	})
}

// NotifyApprovalNeeded sends a notification for approval requests
func (m *Manager) NotifyApprovalNeeded(beadID, title string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeApprovalNeeded,
		Title:   "Approval Required",
		Message: fmt.Sprintf("Bead %s needs approval: %s", beadID, title),
		Data: map[string]interface{}{
			"bead_id": beadID,
		},
	})
}

// NotifyAgentStuck sends a notification when an agent appears stuck
func (m *Manager) NotifyAgentStuck(agentName, agentID, task string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeError,
		Title:   "Agent Stuck",
		Message: fmt.Sprintf("Agent %s appears stuck on: %s", agentName, task),
		Data: map[string]interface{}{
			"agent_name": agentName,
			"agent_id":   agentID,
			"task":       task,
		},
	})
}

// NotifyAgentError sends a notification when an agent encounters an error
func (m *Manager) NotifyAgentError(agentName, agentID, errorMsg string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeError,
		Title:   "Agent Error",
		Message: fmt.Sprintf("Agent %s failed: %s", agentName, errorMsg),
		Data: map[string]interface{}{
			"agent_name": agentName,
			"agent_id":   agentID,
			"error":      errorMsg,
		},
	})
}

// NotifyRateLimit sends a notification for rate limit warnings
func (m *Manager) NotifyRateLimit(remainingTokens int, resetTime string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeRateLimit,
		Title:   "Rate Limit Warning",
		Message: fmt.Sprintf("API rate limit approaching. Remaining: %d tokens. Resets: %s", remainingTokens, resetTime),
		Data: map[string]interface{}{
			"remaining_tokens": remainingTokens,
			"reset_time":       resetTime,
		},
	})
}

// NotifyInfo sends a general informational notification
func (m *Manager) NotifyInfo(title, message string) error {
	return m.Notify(Notification{
		Type:    NotificationTypeInfo,
		Title:   title,
		Message: message,
	})
}
