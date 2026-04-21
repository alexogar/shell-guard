package types

import "time"

type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusActive   SessionStatus = "active"
	SessionStatusClosed   SessionStatus = "closed"
	SessionStatusErrored  SessionStatus = "errored"
)

type CommandStatus string

const (
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusCompleted CommandStatus = "completed"
)

type StorageType string

const (
	StorageTypeSQLite StorageType = "sqlite"
	StorageTypeFile   StorageType = "file"
)

type Session struct {
	ID            int64         `json:"id"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ShellPath     string        `json:"shell_path"`
	ShellPID      int           `json:"shell_pid"`
	WorkspaceRoot string        `json:"workspace_root"`
	CurrentCWD    string        `json:"current_cwd"`
	Status        SessionStatus `json:"status"`
	Hostname      string        `json:"hostname"`
	Username      string        `json:"username"`
}

type CommandRecord struct {
	ID                int64         `json:"id"`
	SessionID         int64         `json:"session_id"`
	RawCommand        string        `json:"raw_command"`
	NormalizedCommand string        `json:"normalized_command"`
	CommandFamily     string        `json:"command_family"`
	CWD               string        `json:"cwd"`
	RepoRoot          string        `json:"repo_root"`
	GitBranch         string        `json:"git_branch"`
	GitDirty          bool          `json:"git_dirty"`
	StartedAt         time.Time     `json:"started_at"`
	FinishedAt        *time.Time    `json:"finished_at,omitempty"`
	DurationMS        int64         `json:"duration_ms"`
	ExitCode          *int          `json:"exit_code,omitempty"`
	Status            CommandStatus `json:"status"`
	SummaryShort      string        `json:"summary_short"`
	RawOutputID       *int64        `json:"raw_output_id,omitempty"`
	RedactedOutputID  *int64        `json:"redacted_output_id,omitempty"`
}

type OutputRecord struct {
	ID          int64       `json:"id"`
	CommandID   int64       `json:"command_id"`
	Kind        string      `json:"kind"`
	StorageType StorageType `json:"storage_type"`
	Body        string      `json:"body,omitempty"`
	Path        string      `json:"path,omitempty"`
	SizeBytes   int64       `json:"size_bytes"`
	CreatedAt   time.Time   `json:"created_at"`
}

type StateSnapshot struct {
	ID               int64     `json:"id"`
	SessionID        int64     `json:"session_id"`
	CurrentCWD       string    `json:"current_cwd"`
	RepoRoot         string    `json:"repo_root"`
	GitBranch        string    `json:"git_branch"`
	GitDirty         bool      `json:"git_dirty"`
	LastCommandID    *int64    `json:"last_command_id,omitempty"`
	LastExitCode     *int      `json:"last_exit_code,omitempty"`
	LastSummaryShort string    `json:"last_summary_short"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type StateView struct {
	SessionStatus    SessionStatus `json:"session_status"`
	CurrentCWD       string        `json:"current_cwd"`
	RepoRoot         string        `json:"repo_root"`
	GitBranch        string        `json:"git_branch"`
	GitDirty         bool          `json:"git_dirty"`
	LastExitCode     *int          `json:"last_exit_code,omitempty"`
	LastSummaryShort string        `json:"last_summary_short"`
}

type RecentCommand struct {
	ID           int64     `json:"id"`
	RawCommand   string    `json:"raw_command"`
	ExitCode     int       `json:"exit_code"`
	SummaryShort string    `json:"summary_short"`
	StartedAt    time.Time `json:"started_at"`
}
