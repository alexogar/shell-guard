package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"shell-guard/internal/config"
	"shell-guard/internal/types"
)

type Store struct {
	db *sql.DB
}

func Open(cfg config.Config) (*Store, error) {
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return nil, fmt.Errorf("set journal mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.runMigrations(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) MarkActiveSessionsErrored(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET status = 'errored', updated_at = ?
		WHERE status IN ('starting', 'active')
	`, time.Now().UTC())
	return err
}

func (s *Store) CreateSession(ctx context.Context, session types.Session) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (
			created_at, updated_at, shell_path, shell_pid, workspace_root, current_cwd, status, hostname, username
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		session.CreatedAt.UTC(),
		session.UpdatedAt.UTC(),
		session.ShellPath,
		session.ShellPID,
		session.WorkspaceRoot,
		session.CurrentCWD,
		string(session.Status),
		session.Hostname,
		session.Username,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateSession(ctx context.Context, session *types.Session) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET updated_at = ?, shell_pid = ?, workspace_root = ?, current_cwd = ?, status = ?
		WHERE id = ?
	`,
		session.UpdatedAt.UTC(),
		session.ShellPID,
		session.WorkspaceRoot,
		session.CurrentCWD,
		string(session.Status),
		session.ID,
	)
	return err
}

func (s *Store) GetLatestSession(ctx context.Context) (*types.Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, shell_path, shell_pid, workspace_root, current_cwd, status, hostname, username
		FROM sessions
		ORDER BY id DESC
		LIMIT 1
	`)
	return scanSession(row)
}

func (s *Store) CreateCommandStart(ctx context.Context, command types.CommandRecord) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO commands (
			session_id, raw_command, normalized_command, command_family, parser_used, cwd, repo_root, git_branch, git_dirty,
			started_at, status, summary_short, structured_summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		command.SessionID,
		command.RawCommand,
		command.NormalizedCommand,
		command.CommandFamily,
		command.ParserUsed,
		command.CWD,
		command.RepoRoot,
		command.GitBranch,
		boolToInt(command.GitDirty),
		command.StartedAt.UTC(),
		string(command.Status),
		command.SummaryShort,
		command.StructuredSummary,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CompleteCommand(ctx context.Context, command *types.CommandRecord) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE commands
		SET parser_used = ?, cwd = ?, repo_root = ?, git_branch = ?, git_dirty = ?, finished_at = ?, duration_ms = ?, exit_code = ?, status = ?, summary_short = ?, structured_summary = ?, raw_output_id = ?, redacted_output_id = ?
		WHERE id = ?
	`,
		command.ParserUsed,
		command.CWD,
		command.RepoRoot,
		command.GitBranch,
		boolToInt(command.GitDirty),
		command.FinishedAt.UTC(),
		command.DurationMS,
		intPtrValue(command.ExitCode),
		string(command.Status),
		command.SummaryShort,
		command.StructuredSummary,
		int64PtrValue(command.RawOutputID),
		int64PtrValue(command.RedactedOutputID),
		command.ID,
	)
	return err
}

func (s *Store) InsertOutput(ctx context.Context, output types.OutputRecord) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO outputs (command_id, kind, storage_type, body, path, size_bytes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		output.CommandID,
		output.Kind,
		string(output.StorageType),
		output.Body,
		output.Path,
		output.SizeBytes,
		output.CreatedAt.UTC(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpsertStateSnapshot(ctx context.Context, snapshot types.StateSnapshot) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO state_snapshots (
			session_id, current_cwd, repo_root, git_branch, git_dirty, last_command_id, last_exit_code, last_summary_short, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			current_cwd = excluded.current_cwd,
			repo_root = excluded.repo_root,
			git_branch = excluded.git_branch,
			git_dirty = excluded.git_dirty,
			last_command_id = excluded.last_command_id,
			last_exit_code = excluded.last_exit_code,
			last_summary_short = excluded.last_summary_short,
			updated_at = excluded.updated_at
	`,
		snapshot.SessionID,
		snapshot.CurrentCWD,
		snapshot.RepoRoot,
		snapshot.GitBranch,
		boolToInt(snapshot.GitDirty),
		int64PtrValue(snapshot.LastCommandID),
		intPtrValue(snapshot.LastExitCode),
		snapshot.LastSummaryShort,
		snapshot.UpdatedAt.UTC(),
	)
	return err
}

func (s *Store) GetStateView(ctx context.Context, activeStatus types.SessionStatus) (*types.StateView, error) {
	var (
		currentCWD       sql.NullString
		repoRoot         sql.NullString
		gitBranch        sql.NullString
		gitDirty         sql.NullInt64
		lastExitCode     sql.NullInt64
		lastSummaryShort sql.NullString
	)

	row := s.db.QueryRowContext(ctx, `
		SELECT current_cwd, repo_root, git_branch, git_dirty, last_exit_code, last_summary_short
		FROM state_snapshots
		ORDER BY updated_at DESC
		LIMIT 1
	`)
	err := row.Scan(&currentCWD, &repoRoot, &gitBranch, &gitDirty, &lastExitCode, &lastSummaryShort)
	if err == sql.ErrNoRows {
		return &types.StateView{SessionStatus: activeStatus}, nil
	}
	if err != nil {
		return nil, err
	}

	view := &types.StateView{
		SessionStatus:    activeStatus,
		CurrentCWD:       currentCWD.String,
		RepoRoot:         repoRoot.String,
		GitBranch:        gitBranch.String,
		GitDirty:         gitDirty.Int64 == 1,
		LastSummaryShort: lastSummaryShort.String,
	}
	if !lastExitCode.Valid {
		return view, nil
	}
	exit := int(lastExitCode.Int64)
	view.LastExitCode = &exit
	return view, nil
}

func (s *Store) ListRecentCommands(ctx context.Context, limit int) ([]types.RecentCommand, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, raw_command, COALESCE(exit_code, 0), summary_short, started_at
		FROM commands
		WHERE status = 'completed'
		ORDER BY started_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []types.RecentCommand
	for rows.Next() {
		var item types.RecentCommand
		if err := rows.Scan(&item.ID, &item.RawCommand, &item.ExitCode, &item.SummaryShort, &item.StartedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) runMigrations(ctx context.Context) error {
	statements := strings.Split(schemaSQL, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("run migration statement: %w", err)
		}
	}
	for _, stmt := range []string{
		`ALTER TABLE commands ADD COLUMN parser_used TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE commands ADD COLUMN structured_summary TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("run compatibility migration: %w", err)
		}
	}
	return nil
}

func scanSession(row *sql.Row) (*types.Session, error) {
	session := &types.Session{}
	var status string
	err := row.Scan(
		&session.ID,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.ShellPath,
		&session.ShellPID,
		&session.WorkspaceRoot,
		&session.CurrentCWD,
		&status,
		&session.Hostname,
		&session.Username,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	session.Status = types.SessionStatus(status)
	return session, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intPtrValue(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func int64PtrValue(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func writeFile(path string, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
