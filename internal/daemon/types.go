package daemon

import "time"

type StartOptions struct {
	DaemonPath   string
	SocketPath   string
	ConfigPath   string
	Verbose      bool
	WaitForReady bool
	ReadyTimeout time.Duration
	Background   bool
}

type StartResult struct {
	Success   bool      `json:"success"`
	PID       int       `json:"pid,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Ready     bool      `json:"ready"`
}

type StopResult struct {
	Success   bool      `json:"success"`
	PID       int       `json:"pid,omitempty"`
	StoppedAt time.Time `json:"stopped_at"`
	Error     string    `json:"error,omitempty"`
}
