# ShellGuard

ShellGuard is a local daemon plus CLI that runs one managed interactive shell session and records compact command history for assistant-friendly state queries.

## MVP Features

- explicit daemon lifecycle with `shg daemon run`
- one managed `bash` or `zsh` PTY session
- marker-based command capture
- SQLite metadata store
- file spillover for large command output
- compact `shg state` and `shg recent` commands

## Requirements

- Go 1.22+
- macOS or Linux
- `bash` or `zsh`

## Build

```bash
go build ./cmd/shg
go build ./cmd/shellguardd
```

## Test

```bash
go test ./...
go vet ./...
```

## GitHub Actions

The repository includes two workflows:

- `ci`: runs `gofmt`, `go vet`, `go test`, and binary builds on every push and pull request on Linux and macOS.
- `release`: runs on tags matching `v*` and publishes GitHub release archives with both binaries for `darwin` and `linux` on `amd64` and `arm64`.

Create a release with:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Run

Start the daemon in one terminal:

```bash
go run ./cmd/shellguardd
```

Start a managed shell in another:

```bash
go run ./cmd/shg session start
```

Inspect state from a third terminal:

```bash
go run ./cmd/shg state
go run ./cmd/shg recent
go run ./cmd/shg session status
```

## Configuration

ShellGuard uses a data directory under `~/.shellguard` by default.

Environment overrides:

- `SHG_HOME`
- `SHG_SOCKET`
- `SHG_DB`
- `SHG_OUTPUT_DIR`
- `SHG_SHELL`
- `SHG_INLINE_OUTPUT_LIMIT`

## Notes

- The current slice intentionally supports only one active session.
- Parser, policy, redaction, and process-awareness work are planned but mostly stubbed at the architecture level in this slice.
