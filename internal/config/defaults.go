package config

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			HeartbeatInterval:   "2m",
			BootCheckInterval:   "5m",
			StuckTimeout:        "10m",
			MaxConcurrentAgents: 5,
		},
		Underboss: UnderbossConfig{
			Personality:      "efficient mob underboss",
			ApprovalRequired: true,
			HistoryMode:      "hybrid",
		},
		Soldati: SoldatiConfig{
			AutoName:       true,
			DefaultTimeout: "30m",
		},
		Associates: AssociatesConfig{
			Timeout:       "10m",
			MaxPerSoldati: 3,
		},
		Notifications: NotificationsConfig{
			Terminal:        true,
			SummaryInterval: "1h",
		},
		Safety: SafetyConfig{
			BranchPrefix:     "mob/",
			CommandBlacklist: []string{"sudo", "rm -rf"},
			RequireReview:    true,
		},
		Logging: LoggingConfig{
			Level:     "info",
			Format:    "dual",
			Retention: "7d",
		},
	}
}
