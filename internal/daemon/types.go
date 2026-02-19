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

type StatusResult struct {
	Status    string    `json:"status"`
	Running   bool      `json:"running"`
	Ready     bool      `json:"ready"`
	PID       int       `json:"pid,omitempty"`
	Version   string    `json:"version,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}
