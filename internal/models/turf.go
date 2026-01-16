package models

// Turf represents a registered project
type Turf struct {
	Name       string `toml:"name"`
	Path       string `toml:"path"`
	MainBranch string `toml:"main_branch"`
}

// TurfsConfig holds all registered turfs
type TurfsConfig struct {
	Turfs []Turf `toml:"turf"`
}
