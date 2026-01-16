package models

import "time"

// SoldatiStats tracks performance metrics
type SoldatiStats struct {
	TasksCompleted int     `toml:"tasks_completed"`
	TasksFailed    int     `toml:"tasks_failed"`
	SuccessRate    float64 `toml:"success_rate"`
}

// Soldati represents a named, persistent worker
type Soldati struct {
	Name        string       `toml:"name"`
	CreatedAt   time.Time    `toml:"created_at"`
	LastActive  time.Time    `toml:"last_active"`
	Stats       SoldatiStats `toml:"stats"`
	Turfs       []string     `toml:"turfs,omitempty"`        // assigned turfs, empty = all turfs
	PrimaryTurf string       `toml:"primary_turf,omitempty"` // preferred turf
}
