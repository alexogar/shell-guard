package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"shell-guard/internal/config"
	"shell-guard/internal/session"
	"shell-guard/internal/store"
	"shell-guard/internal/types"
)

func TestHandleStateUsesStoredSessionStatus(t *testing.T) {
	server, st := newTestServer(t)
	now := time.Now().UTC()

	sessionID, err := st.CreateSession(context.Background(), types.Session{
		CreatedAt:     now,
		UpdatedAt:     now,
		ShellPath:     "/bin/zsh",
		ShellPID:      99,
		WorkspaceRoot: "/tmp/project",
		CurrentCWD:    "/tmp/project",
		Status:        types.SessionStatusClosed,
		Hostname:      "host",
		Username:      "user",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	exitCode := 0
	if err := st.UpsertStateSnapshot(context.Background(), types.StateSnapshot{
		SessionID:        sessionID,
		CurrentCWD:       "/tmp/project",
		RepoRoot:         "/tmp/project",
		GitBranch:        "main",
		GitDirty:         false,
		LastExitCode:     &exitCode,
		LastSummaryShort: "cwd is /tmp/project",
		UpdatedAt:        now,
	}); err != nil {
		t.Fatalf("UpsertStateSnapshot failed: %v", err)
	}

	resp := readServerResponse(t, func(serverConn net.Conn) {
		defer serverConn.Close()
		server.handleState(serverConn)
	})

	if !resp.OK || resp.State == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.State.SessionStatus != types.SessionStatusClosed {
		t.Fatalf("expected closed status, got %q", resp.State.SessionStatus)
	}
	if resp.State.LastSummaryShort != "cwd is /tmp/project" {
		t.Fatalf("unexpected summary: %q", resp.State.LastSummaryShort)
	}
}

func TestHandleRecentUsesDefaultLimit(t *testing.T) {
	server, st := newTestServer(t)
	now := time.Now().UTC()

	sessionID, err := st.CreateSession(context.Background(), types.Session{
		CreatedAt:     now,
		UpdatedAt:     now,
		ShellPath:     "/bin/zsh",
		ShellPID:      99,
		WorkspaceRoot: "/tmp/project",
		CurrentCWD:    "/tmp/project",
		Status:        types.SessionStatusClosed,
		Hostname:      "host",
		Username:      "user",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	for i := 0; i < 12; i++ {
		commandID, err := st.CreateCommandStart(context.Background(), types.CommandRecord{
			SessionID:         sessionID,
			RawCommand:        "pwd",
			NormalizedCommand: "pwd",
			CommandFamily:     "pwd",
			CWD:               "/tmp/project",
			StartedAt:         now.Add(time.Duration(i) * time.Second),
			Status:            types.CommandStatusRunning,
		})
		if err != nil {
			t.Fatalf("CreateCommandStart failed: %v", err)
		}
		exitCode := 0
		finishedAt := now.Add(time.Duration(i) * time.Second)
		if err := st.CompleteCommand(context.Background(), &types.CommandRecord{
			ID:           commandID,
			CWD:          "/tmp/project",
			RepoRoot:     "/tmp/project",
			GitBranch:    "main",
			FinishedAt:   &finishedAt,
			DurationMS:   10,
			ExitCode:     &exitCode,
			Status:       types.CommandStatusCompleted,
			SummaryShort: "cwd is /tmp/project",
		}); err != nil {
			t.Fatalf("CompleteCommand failed: %v", err)
		}
	}

	resp := readServerResponse(t, func(serverConn net.Conn) {
		defer serverConn.Close()
		server.handleRecent(serverConn, Request{})
	})

	if !resp.OK {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Recent) != 10 {
		t.Fatalf("expected default limit 10, got %d", len(resp.Recent))
	}
}

func TestCurrentStatusNil(t *testing.T) {
	if got := currentStatus(nil); got != "" {
		t.Fatalf("expected empty status, got %q", got)
	}
}

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	home := t.TempDir()
	cfg := config.Config{
		HomeDir:           home,
		SocketPath:        filepath.Join(home, "shellguard.sock"),
		DBPath:            filepath.Join(home, "shellguard.db"),
		OutputDir:         filepath.Join(home, "outputs"),
		ShellPath:         "/bin/zsh",
		InlineOutputLimit: 1024,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open store failed: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	manager, err := session.NewManager(cfg, st)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return NewServer(cfg, st, manager), st
}

func readServerResponse(t *testing.T, invoke func(serverConn net.Conn)) Response {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		invoke(serverConn)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	_ = clientConn.Close()
	<-done

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}
