package session

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"

	"shell-guard/internal/config"
	"shell-guard/internal/parsers"
	"shell-guard/internal/state"
	"shell-guard/internal/store"
	"shell-guard/internal/types"
)

const markerPrefix = "\n__SHG__|"

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type Manager struct {
	cfg    config.Config
	store  *store.Store
	reg    *parsers.Registry
	mu     sync.Mutex
	active *ManagedSession
}

type ManagedSession struct {
	cfg            config.Config
	store          *store.Store
	registry       *parsers.Registry
	mu             sync.Mutex
	session        types.Session
	cmd            *exec.Cmd
	ptmx           *os.File
	conn           net.Conn
	done           chan struct{}
	parser         markerStream
	current        *runningCommand
	readErr        error
	integrationDir string
}

type runningCommand struct {
	record types.CommandRecord
	spool  *spillBuffer
}

type markerStream struct {
	pending []byte
}

type marker struct {
	Kind     string
	CWD      string
	Command  string
	ExitCode int
}

type spillBuffer struct {
	limit     int64
	outputDir string
	buf       strings.Builder
	file      *os.File
	path      string
	size      int64
}

func NewManager(cfg config.Config, st *store.Store) (*Manager, error) {
	return &Manager{cfg: cfg, store: st, reg: parsers.DefaultRegistry()}, nil
}

func (m *Manager) StartSession(shellPath, cwd string, rows, cols int) (*ManagedSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active != nil && m.active.Snapshot().Status == types.SessionStatusActive {
		return nil, errors.New("an active session already exists")
	}

	info, err := buildSessionMetadata(shellPath, cwd)
	if err != nil {
		return nil, err
	}
	info.Status = types.SessionStatusStarting
	now := time.Now().UTC()
	info.CreatedAt = now
	info.UpdatedAt = now

	id, err := m.store.CreateSession(context.Background(), info)
	if err != nil {
		return nil, fmt.Errorf("create session row: %w", err)
	}
	info.ID = id

	integrationDir := filepath.Join(m.cfg.HomeDir, "sessions", strconv.FormatInt(id, 10))
	if err := os.MkdirAll(integrationDir, 0o755); err != nil {
		return nil, fmt.Errorf("create integration dir: %w", err)
	}

	cmd, err := prepareShellCommand(shellPath, cwd, integrationDir)
	if err != nil {
		return nil, err
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}
	if rows > 0 && cols > 0 {
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}

	info.ShellPID = cmd.Process.Pid
	info.Status = types.SessionStatusActive
	info.UpdatedAt = time.Now().UTC()
	if err := m.store.UpdateSession(context.Background(), &info); err != nil {
		_ = ptmx.Close()
		return nil, fmt.Errorf("update session active: %w", err)
	}

	managed := &ManagedSession{
		cfg:            m.cfg,
		store:          m.store,
		registry:       m.reg,
		session:        info,
		cmd:            cmd,
		ptmx:           ptmx,
		done:           make(chan struct{}),
		integrationDir: integrationDir,
	}
	m.active = managed

	go managed.readLoop()
	go func() {
		err := cmd.Wait()
		managed.finish(err)
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.active == managed {
			m.active = nil
		}
	}()

	return managed, nil
}

func (m *Manager) ActiveSession() *ManagedSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func (s *ManagedSession) Snapshot() *types.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.session
	return &copy
}

func (s *ManagedSession) Attach(conn net.Conn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(s.ptmx, conn)
		close(copyDone)
	}()

	select {
	case <-s.done:
	case <-copyDone:
	}

	s.mu.Lock()
	if s.conn == conn {
		s.conn = nil
	}
	s.mu.Unlock()
}

func (s *ManagedSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.handleChunk(buf[:n])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.readErr = err
			}
			s.flushPendingOutput()
			return
		}
	}
}

func (s *ManagedSession) handleChunk(chunk []byte) {
	s.parser.pending = append(s.parser.pending, chunk...)

	for {
		idx := strings.Index(string(s.parser.pending), markerPrefix)
		if idx == -1 {
			flush := len(s.parser.pending) - len(markerPrefix)
			if flush > 0 {
				s.forwardOutput(s.parser.pending[:flush])
				s.parser.pending = append([]byte{}, s.parser.pending[flush:]...)
			}
			return
		}

		if idx > 0 {
			s.forwardOutput(s.parser.pending[:idx])
			s.parser.pending = append([]byte{}, s.parser.pending[idx:]...)
			idx = 0
		}

		if len(s.parser.pending) == 0 || s.parser.pending[0] != '\n' {
			return
		}

		lineEnd := bytesIndexByte(s.parser.pending[1:], '\n')
		if lineEnd == -1 {
			return
		}
		line := string(s.parser.pending[1 : lineEnd+1])
		s.parser.pending = append([]byte{}, s.parser.pending[lineEnd+2:]...)

		marker, ok := parseMarkerLine(line)
		if !ok {
			s.forwardOutput([]byte("\n" + line + "\n"))
			continue
		}
		s.handleMarker(marker)
	}
}

func (s *ManagedSession) flushPendingOutput() {
	if len(s.parser.pending) == 0 {
		return
	}
	s.forwardOutput(s.parser.pending)
	s.parser.pending = nil
}

func (s *ManagedSession) forwardOutput(chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	s.mu.Lock()
	conn := s.conn
	current := s.current
	s.mu.Unlock()

	if current != nil {
		_, _ = current.spool.Write(chunk)
	}
	if conn != nil {
		_, _ = conn.Write(chunk)
	}
}

func (s *ManagedSession) handleMarker(mark marker) {
	switch mark.Kind {
	case "BEGIN":
		s.beginCommand(mark)
	case "END":
		s.endCommand(mark)
	}
}

func (s *ManagedSession) beginCommand(mark marker) {
	record := types.CommandRecord{
		SessionID:         s.session.ID,
		RawCommand:        mark.Command,
		NormalizedCommand: strings.TrimSpace(mark.Command),
		CommandFamily:     state.GuessCommandFamily(mark.Command),
		CWD:               mark.CWD,
		StartedAt:         time.Now().UTC(),
		Status:            types.CommandStatusRunning,
		SummaryShort:      "",
	}
	id, err := s.store.CreateCommandStart(context.Background(), record)
	if err != nil {
		return
	}
	record.ID = id

	spool := &spillBuffer{
		limit:     s.cfg.InlineOutputLimit,
		outputDir: s.cfg.OutputDir,
	}

	s.mu.Lock()
	s.current = &runningCommand{record: record, spool: spool}
	s.session.CurrentCWD = mark.CWD
	s.mu.Unlock()
}

func (s *ManagedSession) endCommand(mark marker) {
	s.mu.Lock()
	current := s.current
	s.current = nil
	s.session.CurrentCWD = mark.CWD
	s.session.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()

	if current == nil {
		return
	}

	finishedAt := time.Now().UTC()
	repoRoot, branch, dirty := discoverRepo(mark.CWD)
	exitCode := mark.ExitCode
	current.record.CWD = mark.CWD
	current.record.RepoRoot = repoRoot
	current.record.GitBranch = branch
	current.record.GitDirty = dirty
	current.record.FinishedAt = &finishedAt
	current.record.DurationMS = finishedAt.Sub(current.record.StartedAt).Milliseconds()
	current.record.ExitCode = &exitCode
	current.record.Status = types.CommandStatusCompleted
	current.record.SummaryShort = state.SummarizeCommand(current.record.RawCommand, exitCode)

	storageType, body, path, size, err := current.spool.Finalize()
	if err == nil {
		output := types.OutputRecord{
			CommandID:   current.record.ID,
			Kind:        "raw",
			StorageType: storageType,
			Body:        body,
			Path:        path,
			SizeBytes:   size,
			CreatedAt:   time.Now().UTC(),
		}
		outputID, outputErr := s.store.InsertOutput(context.Background(), output)
		if outputErr == nil {
			current.record.RawOutputID = &outputID
		}
		s.applyParserResult(&current.record, body, path, storageType)
	}

	_ = s.store.CompleteCommand(context.Background(), &current.record)
	_ = s.store.UpsertStateSnapshot(context.Background(), types.StateSnapshot{
		SessionID:        s.session.ID,
		CurrentCWD:       mark.CWD,
		RepoRoot:         repoRoot,
		GitBranch:        branch,
		GitDirty:         dirty,
		LastCommandID:    &current.record.ID,
		LastExitCode:     &exitCode,
		LastSummaryShort: current.record.SummaryShort,
		UpdatedAt:        time.Now().UTC(),
	})

	s.mu.Lock()
	s.session.WorkspaceRoot = repoRootOrCWD(repoRoot, mark.CWD)
	s.session.UpdatedAt = time.Now().UTC()
	s.mu.Unlock()
	_ = s.store.UpdateSession(context.Background(), &s.session)
}

func (s *ManagedSession) applyParserResult(record *types.CommandRecord, body, path string, storageType types.StorageType) {
	if s.registry == nil || record.ExitCode == nil {
		return
	}

	output := body
	if storageType == types.StorageTypeFile && path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		output = string(data)
	}

	commandName, args := splitCommand(record.RawCommand)
	result, ok := s.registry.Parse(parsers.Context{
		RawCommand:  record.RawCommand,
		CommandName: commandName,
		Args:        args,
		CWD:         record.CWD,
		RepoRoot:    record.RepoRoot,
		GitBranch:   record.GitBranch,
		GitDirty:    record.GitDirty,
		ExitCode:    *record.ExitCode,
		Output:      normalizeParserOutput(output),
		DurationMS:  record.DurationMS,
	})
	if !ok {
		return
	}

	record.ParserUsed = result.Name
	record.SummaryShort = result.SummaryShort
	record.StructuredSummary = result.StructuredSummary
}

func (s *ManagedSession) finish(waitErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := types.SessionStatusClosed
	if waitErr != nil || s.readErr != nil {
		status = types.SessionStatusErrored
	}
	s.session.Status = status
	s.session.UpdatedAt = time.Now().UTC()
	_ = s.store.UpdateSession(context.Background(), &s.session)

	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	close(s.done)
}

func buildSessionMetadata(shellPath, cwd string) (types.Session, error) {
	host, _ := os.Hostname()
	currentUser, _ := user.Current()
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return types.Session{}, fmt.Errorf("resolve cwd: %w", err)
		}
	}
	repoRoot, _, _ := discoverRepo(cwd)
	username := ""
	if currentUser != nil {
		username = currentUser.Username
	}

	return types.Session{
		ShellPath:     shellPath,
		WorkspaceRoot: repoRootOrCWD(repoRoot, cwd),
		CurrentCWD:    cwd,
		Hostname:      host,
		Username:      username,
	}, nil
}

func prepareShellCommand(shellPath, cwd, integrationDir string) (*exec.Cmd, error) {
	base := filepath.Base(shellPath)
	switch base {
	case "bash":
		rcPath := filepath.Join(integrationDir, ".bashrc")
		if err := os.WriteFile(rcPath, []byte(bashIntegrationScript), 0o644); err != nil {
			return nil, fmt.Errorf("write bash integration: %w", err)
		}
		cmd := exec.Command(shellPath, "--noprofile", "--rcfile", rcPath, "-i")
		cmd.Dir = cwd
		cmd.Env = os.Environ()
		return cmd, nil
	case "zsh":
		rcPath := filepath.Join(integrationDir, ".zshrc")
		if err := os.WriteFile(rcPath, []byte(zshIntegrationScript), 0o644); err != nil {
			return nil, fmt.Errorf("write zsh integration: %w", err)
		}
		cmd := exec.Command(shellPath, "-i")
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "ZDOTDIR="+integrationDir)
		return cmd, nil
	default:
		return nil, fmt.Errorf("unsupported shell: %s", shellPath)
	}
}

func parseMarkerLine(line string) (marker, bool) {
	line = strings.TrimSpace(line)
	parts := strings.Split(line, "|")
	if len(parts) < 4 || parts[0] != "__SHG__" {
		return marker{}, false
	}
	switch parts[1] {
	case "BEGIN":
		cwd, err1 := decodeBase64(parts[2])
		command, err2 := decodeBase64(parts[3])
		if err1 != nil || err2 != nil {
			return marker{}, false
		}
		return marker{Kind: "BEGIN", CWD: cwd, Command: command}, true
	case "END":
		cwd, err := decodeBase64(parts[3])
		if err != nil {
			return marker{}, false
		}
		exitCode, err := strconv.Atoi(parts[2])
		if err != nil {
			return marker{}, false
		}
		return marker{Kind: "END", CWD: cwd, ExitCode: exitCode}, true
	default:
		return marker{}, false
	}
}

func decodeBase64(value string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func bytesIndexByte(buf []byte, needle byte) int {
	for i, b := range buf {
		if b == needle {
			return i
		}
	}
	return -1
}

func discoverRepo(cwd string) (repoRoot, branch string, dirty bool) {
	repoRoot = strings.TrimSpace(runGit(cwd, "rev-parse", "--show-toplevel"))
	if repoRoot == "" {
		return "", "", false
	}
	branch = strings.TrimSpace(runGit(cwd, "branch", "--show-current"))
	dirty = strings.TrimSpace(runGit(cwd, "status", "--porcelain")) != ""
	return repoRoot, branch, dirty
}

func runGit(cwd string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func repoRootOrCWD(repoRoot, cwd string) string {
	if repoRoot != "" {
		return repoRoot
	}
	return cwd
}

func splitCommand(raw string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", nil
	}
	return filepath.Base(fields[0]), fields[1:]
}

func normalizeParserOutput(output string) string {
	output = strings.ReplaceAll(output, "\r", "")
	output = ansiEscapePattern.ReplaceAllString(output, "")
	lines := strings.Split(output, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "", "%", "shg$":
			continue
		}
		cleaned = append(cleaned, strings.TrimRight(line, " \t"))
	}
	return strings.Join(cleaned, "\n")
}

func (s *spillBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if s.file == nil && s.size+int64(len(p)) <= s.limit {
		n, err := s.buf.WriteString(string(p))
		s.size += int64(n)
		return n, err
	}
	if s.file == nil {
		file, err := os.CreateTemp(s.outputDir, "cmd-output-*.log")
		if err != nil {
			return 0, err
		}
		s.file = file
		s.path = file.Name()
		if _, err := s.file.WriteString(s.buf.String()); err != nil {
			return 0, err
		}
		s.buf.Reset()
	}
	n, err := s.file.Write(p)
	s.size += int64(n)
	return n, err
}

func (s *spillBuffer) Finalize() (types.StorageType, string, string, int64, error) {
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			return "", "", "", 0, err
		}
		return types.StorageTypeFile, "", s.path, s.size, nil
	}
	return types.StorageTypeSQLite, s.buf.String(), "", s.size, nil
}

const bashIntegrationScript = `
case $- in
  *i*) ;;
  *) return ;;
esac

__shg_b64() {
  printf '%s' "$1" | base64 | tr -d '\r\n'
}

__shg_in_command=0

__shg_preexec() {
  if [ "${__shg_in_command}" -eq 0 ]; then
    __shg_in_command=1
    printf '\n__SHG__|BEGIN|%s|%s\n' "$(__shg_b64 "$PWD")" "$(__shg_b64 "$BASH_COMMAND")"
  fi
}

__shg_precmd() {
  local shg_exit=$?
  if [ "${__shg_in_command}" -eq 1 ]; then
    printf '\n__SHG__|END|%s|%s\n' "${shg_exit}" "$(__shg_b64 "$PWD")"
    __shg_in_command=0
  fi
}

trap '__shg_preexec' DEBUG
PROMPT_COMMAND='__shg_precmd'
PS1='shg$ '
`

const zshIntegrationScript = `
autoload -Uz add-zsh-hook

__shg_b64() {
  printf '%s' "$1" | base64 | tr -d '\r\n'
}

typeset -g __shg_in_command=0

__shg_preexec() {
  __shg_in_command=1
  printf '\n__SHG__|BEGIN|%s|%s\n' "$(__shg_b64 "$PWD")" "$(__shg_b64 "$1")"
}

__shg_precmd() {
  local shg_exit=$?
  if [ "${__shg_in_command}" -eq 1 ]; then
    printf '\n__SHG__|END|%s|%s\n' "${shg_exit}" "$(__shg_b64 "$PWD")"
    __shg_in_command=0
  fi
}

add-zsh-hook preexec __shg_preexec
add-zsh-hook precmd __shg_precmd
PROMPT='shg$ '
`
