package daemon

// Daemon manages the mob orchestration
type Daemon struct {
	pidFile   string
	stateFile string
	running   bool
}

// New creates a new daemon instance
func New(pidFile, stateFile string) *Daemon {
	return &Daemon{
		pidFile:   pidFile,
		stateFile: stateFile,
	}
}
