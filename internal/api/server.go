package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"shell-guard/internal/config"
	"shell-guard/internal/session"
	"shell-guard/internal/store"
	"shell-guard/internal/types"
)

type Server struct {
	cfg     config.Config
	store   *store.Store
	manager *session.Manager
}

func NewServer(cfg config.Config, st *store.Store, manager *session.Manager) *Server {
	return &Server{cfg: cfg, store: st, manager: manager}
}

func (s *Server) Run(ctx context.Context) error {
	_ = os.Remove(s.cfg.SocketPath)
	listener, err := net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}
	defer func() {
		listener.Close()
		_ = os.Remove(s.cfg.SocketPath)
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return context.Canceled
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("accept connection: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: fmt.Sprintf("read request: %v", err)})
		return
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: fmt.Sprintf("decode request: %v", err)})
		return
	}

	switch req.Action {
	case ActionStartSession:
		s.handleStartSession(conn, req)
	case ActionSessionStatus:
		s.handleSessionStatus(conn)
	case ActionGetState:
		s.handleState(conn)
	case ActionListRecentCommands:
		s.handleRecent(conn, req)
	default:
		_ = writeResponse(conn, Response{OK: false, Error: "unknown action"})
	}
}

func (s *Server) handleStartSession(conn net.Conn, req Request) {
	managed, err := s.manager.StartSession(req.Shell, req.CWD, req.Rows, req.Cols)
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
		return
	}

	if err := writeResponse(conn, Response{
		OK:      true,
		Message: "session started",
		Session: managed.Snapshot(),
	}); err != nil {
		return
	}

	managed.Attach(conn)
}

func (s *Server) handleSessionStatus(conn net.Conn) {
	current := s.manager.ActiveSession()
	if current != nil {
		_ = writeResponse(conn, Response{OK: true, Session: current.Snapshot()})
		return
	}

	sessionRow, err := s.store.GetLatestSession(context.Background())
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
		return
	}
	if sessionRow == nil {
		_ = writeResponse(conn, Response{OK: true})
		return
	}
	_ = writeResponse(conn, Response{OK: true, Session: sessionRow})
}

func (s *Server) handleState(conn net.Conn) {
	status := currentStatus(s.manager.ActiveSession())
	if status == "" {
		sessionRow, err := s.store.GetLatestSession(context.Background())
		if err != nil {
			_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
			return
		}
		if sessionRow != nil {
			status = sessionRow.Status
		}
	}

	view, err := s.store.GetStateView(context.Background(), status)
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
		return
	}
	_ = writeResponse(conn, Response{OK: true, State: view})
}

func (s *Server) handleRecent(conn net.Conn, req Request) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	items, err := s.store.ListRecentCommands(context.Background(), limit)
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
		return
	}
	_ = writeResponse(conn, Response{OK: true, Recent: items})
}

func currentStatus(active *session.ManagedSession) types.SessionStatus {
	if active == nil {
		return ""
	}
	return active.Snapshot().Status
}

func writeResponse(w net.Conn, resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
