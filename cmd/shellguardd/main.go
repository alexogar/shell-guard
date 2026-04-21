package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shell-guard/internal/api"
	"shell-guard/internal/config"
	"shell-guard/internal/session"
	"shell-guard/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st, err := store.Open(cfg)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.MarkActiveSessionsErrored(context.Background()); err != nil {
		log.Fatalf("recover sessions: %v", err)
	}

	manager, err := session.NewManager(cfg, st)
	if err != nil {
		log.Fatalf("create session manager: %v", err)
	}

	server := api.NewServer(cfg, st, manager)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("shellguardd listening on %s", cfg.SocketPath)
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("run daemon: %v", err)
	}
}
