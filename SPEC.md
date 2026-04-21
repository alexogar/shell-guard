# ShellGuard MVP Specification

## 1. Overview

**ShellGuard** is a local terminal sidecar that sits in front of a real shell and gives AI assistants a compact, structured view of command execution.

For the MVP, ShellGuard is intentionally narrow:

- it runs locally
- it uses a daemon plus CLI
- it owns one interactive shell session at a time
- it captures command boundaries with injected shell hooks
- it stores compact metadata in SQLite
- it stores large output bodies in files

ShellGuard is **not** a terminal emulator replacement and does **not** use an LLM in the hot path.

CLI naming:

- Product: `ShellGuard`
- Binary: `shg`
- Daemon: `shellguardd`

## 2. Goals

ShellGuard should:

1. Run a real interactive shell through a PTY owned by a local daemon.
2. Capture commands and command results as structured records.
3. Expose compact assistant-facing state instead of raw transcript by default.
4. Keep memory usage bounded while preserving output by reference.
5. Provide a clean base for later parser, policy, redaction, and process-awareness work.

## 3. Non-Goals

The MVP does not need to:

- support multiple concurrent sessions
- support shells beyond `bash` and `zsh`
- auto-start the daemon in the background
- provide rich terminal UI features
- implement broad parser coverage
- implement full policy enforcement or redaction logic beyond extensibility hooks
- support Windows

## 4. MVP Constraints

### 4.1 Session Model

- Exactly one active interactive session is supported at a time.
- Multi-session support is deferred.
- The daemon owns the shell process and PTY lifecycle.

### 4.2 Supported Platforms And Shells

- Target platforms: macOS and Linux
- Supported shells: `bash`, `zsh`
- All other shells are out of scope for MVP

### 4.3 Explicit Lifecycle

The CLI uses explicit flows:

- `shg daemon run`
- `shg session start`
- `shg session status`
- `shg state`
- `shg recent`

The CLI must not silently spawn a background daemon for the user.

## 5. Technical Stack

Use Go for the MVP.

Recommended baseline:

- PTY: a Go PTY library
- CLI: standard library is acceptable for MVP
- DB: SQLite
- IPC: Unix domain socket with JSON request/response
- Logging: structured logs to stderr or file

## 6. Architecture

```text
User / Assistant
        |
        v
      shg CLI
        |
        v
 Unix socket JSON API
        |
        v
   shellguardd daemon
        |
   single PTY session
        |
        v
   real bash/zsh shell
```

Main components:

- `shellguardd`: owns the PTY session, shell lifecycle, command capture, persistence, and API
- `shg`: starts sessions, attaches to the managed shell, and renders compact state
- `store`: persists sessions, commands, outputs, and snapshots in SQLite
- `ptybroker`: starts shells with ShellGuard integration hooks and parses command markers
- `state`: produces compact state snapshots for CLI consumption

## 7. Command Capture Model

### 7.1 Strategy

Command boundaries are detected by injected shell integration, not by prompt heuristics.

ShellGuard writes shell-specific startup scripts that emit machine-readable markers:

- `BEGIN` marker before a command runs
- `END` marker after the command finishes

Each marker carries enough data to reconstruct:

- command text
- current working directory
- exit code for completion markers

### 7.2 Marker Behavior

- Marker lines are written into the PTY stream.
- The daemon strips markers before forwarding normal output to the attached client.
- Normal output between `BEGIN` and `END` markers is attributed to the current command.

### 7.3 Fallback

If markers are missing or malformed:

- the daemon keeps the session alive
- unstructured PTY output is still forwarded to the user
- the daemon records a warning and skips structured command capture for the affected segment

## 8. Data Model

### 8.1 Session

```text
Session {
  id
  created_at
  updated_at
  shell_path
  shell_pid
  workspace_root
  current_cwd
  status            // starting | active | closed | errored
  hostname
  username
}
```

### 8.2 CommandRecord

```text
CommandRecord {
  id
  session_id
  raw_command
  normalized_command
  command_family
  cwd
  repo_root
  git_branch
  git_dirty
  started_at
  finished_at
  duration_ms
  exit_code
  status            // running | completed
  summary_short
  raw_output_id
  redacted_output_id
}
```

### 8.3 OutputRecord

```text
OutputRecord {
  id
  command_id
  kind              // raw | redacted
  storage_type      // sqlite | file
  body
  path
  size_bytes
  created_at
}
```

### 8.4 StateSnapshot

```text
StateSnapshot {
  id
  session_id
  current_cwd
  repo_root
  git_branch
  git_dirty
  last_command_id
  last_exit_code
  last_summary_short
  updated_at
}
```

## 9. Storage Strategy

- SQLite stores metadata and compact state.
- Small output bodies may be stored inline in SQLite.
- Large output bodies spill to files under the ShellGuard data directory.
- `outputs` rows keep the storage reference and byte size.

The implementation should keep output buffering bounded and avoid holding full large outputs in memory when possible.

## 10. IPC Contract

Transport:

- Unix domain socket
- one JSON request and one JSON response per control call

For `session start`, the control response is followed by an attached byte stream on the same socket so the CLI can bridge the user terminal to the daemon-managed PTY.

Required control operations for MVP:

- `StartSession`
- `GetSessionStatus`
- `GetState`
- `ListRecentCommands`

Response style:

- compact JSON
- deterministic field names
- explicit error message on failure

## 11. CLI Design

### 11.1 Required Commands

#### `shg daemon run`

Starts the local daemon and listens on the configured Unix socket.

#### `shg session start`

Starts the single managed shell session and attaches the local terminal to it.

#### `shg session status`

Shows whether a managed session exists and whether it is active.

#### `shg state`

Shows compact current state:

- session status
- cwd
- repo root
- branch
- git dirty state
- last command summary
- last exit code

#### `shg recent`

Shows recent completed commands:

- command id
- command text
- exit code
- short summary
- timestamp

### 11.2 Output Requirements

Default outputs must be:

- compact
- predictable
- copy/paste friendly
- small enough for assistant use

Raw output is available by reference, not as the default CLI surface.

## 12. Repository Layout

```text
/README.md
/SPEC.md
/go.mod

/cmd/shellguardd/main.go
/cmd/shg/main.go

/internal/api/
/internal/config/
/internal/ptybroker/
/internal/session/
/internal/state/
/internal/store/
/internal/types/

/migrations/
```

## 13. Milestones

### Milestone 1: Vertical Slice

Deliver:

- daemon startup
- one PTY-backed shell session
- marker-based command capture
- SQLite persistence
- `shg session start`
- `shg session status`
- `shg state`
- `shg recent`

### Milestone 2: Parser Foundation

Deliver:

- parser registry
- initial deterministic parsers for `pwd`, `git branch --show-current`, `git status`, `ls`, `pytest`

### Milestone 3: Policy And Redaction

Deliver:

- pre-exec policy checks for managed-shell commands
- path-based secret file rules
- assistant-facing redacted output paths

### Milestone 4: Process Awareness And Handoff

Deliver:

- best-effort process observations
- duplicate server heuristics
- `shg ps`
- `shg failures`
- `shg handoff`

## 14. Success Criteria

The MVP is successful if:

1. `shellguardd` starts and creates its working directories.
2. `shg session start` opens a real interactive shell managed by the daemon.
3. Commands are captured with start time, end time, cwd, exit code, and output reference.
4. `shg state` shows compact situational awareness without dumping raw terminal output.
5. `shg recent` shows recent commands in a stable compact format.
6. Large outputs can spill to files while metadata remains queryable through SQLite.

## 15. What To Build First

Build the first working vertical slice now:

- project scaffold
- daemon
- one PTY-backed shell session
- command capture
- SQLite persistence
- `shg state`
- `shg recent`
- README with build and run instructions

Prioritize:

- clear architecture
- correct data flow
- easy iteration
- small outputs
- no unnecessary polish
