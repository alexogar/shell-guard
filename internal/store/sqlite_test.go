package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"shell-guard/internal/config"
	"shell-guard/internal/types"
)

func TestStoreCommandLifecycle(t *testing.T) {
	home := t.TempDir()
	cfg := config.Config{
		HomeDir:           home,
		DBPath:            filepath.Join(home, "shellguard.db"),
		OutputDir:         filepath.Join(home, "outputs"),
		InlineOutputLimit: 1024,
	}
	st, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC()
	sessionID, err := st.CreateSession(context.Background(), types.Session{
		CreatedAt:     now,
		UpdatedAt:     now,
		ShellPath:     "/bin/zsh",
		ShellPID:      123,
		WorkspaceRoot: home,
		CurrentCWD:    home,
		Status:        types.SessionStatusActive,
		Hostname:      "host",
		Username:      "user",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	commandID, err := st.CreateCommandStart(context.Background(), types.CommandRecord{
		SessionID:         sessionID,
		RawCommand:        "pwd",
		NormalizedCommand: "pwd",
		CommandFamily:     "pwd",
		CWD:               home,
		StartedAt:         now,
		Status:            types.CommandStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateCommandStart failed: %v", err)
	}

	outputID, err := st.InsertOutput(context.Background(), types.OutputRecord{
		CommandID:   commandID,
		Kind:        "raw",
		StorageType: types.StorageTypeSQLite,
		Body:        home,
		SizeBytes:   int64(len(home)),
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("InsertOutput failed: %v", err)
	}

	exitCode := 0
	finishedAt := now.Add(100 * time.Millisecond)
	if err := st.CompleteCommand(context.Background(), &types.CommandRecord{
		ID:                commandID,
		CWD:               home,
		RepoRoot:          home,
		GitBranch:         "main",
		GitDirty:          false,
		FinishedAt:        &finishedAt,
		DurationMS:        100,
		ExitCode:          &exitCode,
		Status:            types.CommandStatusCompleted,
		SummaryShort:      "cwd is " + home,
		ParserUsed:        "pwd",
		StructuredSummary: `{"path":"` + home + `"}`,
		RawOutputID:       &outputID,
	}); err != nil {
		t.Fatalf("CompleteCommand failed: %v", err)
	}

	if err := st.UpsertStateSnapshot(context.Background(), types.StateSnapshot{
		SessionID:        sessionID,
		CurrentCWD:       home,
		RepoRoot:         home,
		GitBranch:        "main",
		GitDirty:         false,
		LastCommandID:    &commandID,
		LastExitCode:     &exitCode,
		LastSummaryShort: "cwd is " + home,
		UpdatedAt:        finishedAt,
	}); err != nil {
		t.Fatalf("UpsertStateSnapshot failed: %v", err)
	}

	view, err := st.GetStateView(context.Background(), types.SessionStatusClosed)
	if err != nil {
		t.Fatalf("GetStateView failed: %v", err)
	}
	if view.LastSummaryShort != "cwd is "+home {
		t.Fatalf("unexpected summary: %q", view.LastSummaryShort)
	}

	recent, err := st.ListRecentCommands(context.Background(), 5)
	if err != nil {
		t.Fatalf("ListRecentCommands failed: %v", err)
	}
	if len(recent) != 1 || recent[0].SummaryShort != "cwd is "+home {
		t.Fatalf("unexpected recent commands: %#v", recent)
	}
}
