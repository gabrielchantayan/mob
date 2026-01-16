package config

import "time"

// DefaultAssociateTimeout is the default timeout for associates (10 minutes)
const DefaultAssociateTimeout = 10 * time.Minute

// DefaultAssociateGracePeriod is the grace period after nudge before force kill (1 minute)
const DefaultAssociateGracePeriod = 1 * time.Minute

// Config holds the main mob configuration
type Config struct {
	Daemon        DaemonConfig        `toml:"daemon"`
	Underboss     UnderbossConfig     `toml:"underboss"`
	Soldati       SoldatiConfig       `toml:"soldati"`
	Associates    AssociatesConfig    `toml:"associates"`
	Notifications NotificationsConfig `toml:"notifications"`
	Safety        SafetyConfig        `toml:"safety"`
	Logging       LoggingConfig       `toml:"logging"`
}

type DaemonConfig struct {
	HeartbeatInterval   string `toml:"heartbeat_interval"`
	BootCheckInterval   string `toml:"boot_check_interval"`
	StuckTimeout        string `toml:"stuck_timeout"`
	MaxConcurrentAgents int    `toml:"max_concurrent_agents"`
}

type UnderbossConfig struct {
	Personality      string `toml:"personality"`
	ApprovalRequired bool   `toml:"approval_required"`
	HistoryMode      string `toml:"history_mode"`
}

type SoldatiConfig struct {
	AutoName       bool   `toml:"auto_name"`
	DefaultTimeout string `toml:"default_timeout"`
}

type AssociatesConfig struct {
	Timeout       string `toml:"timeout"`
	MaxPerSoldati int    `toml:"max_per_soldati"`
}

type NotificationsConfig struct {
	Terminal        bool   `toml:"terminal"`
	SummaryInterval string `toml:"summary_interval"`
}

type SafetyConfig struct {
	BranchPrefix     string   `toml:"branch_prefix"`
	CommandBlacklist []string `toml:"command_blacklist"`
	RequireReview    bool     `toml:"require_review"`
}

type LoggingConfig struct {
	Level     string `toml:"level"`
	Format    string `toml:"format"`
	Retention string `toml:"retention"`
}

// GetAssociateTimeout parses the associate timeout string and returns a duration.
// Returns DefaultAssociateTimeout if the string is empty or invalid.
func (c *AssociatesConfig) GetAssociateTimeout() time.Duration {
	if c.Timeout == "" {
		return DefaultAssociateTimeout
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return DefaultAssociateTimeout
	}
	return d
}
