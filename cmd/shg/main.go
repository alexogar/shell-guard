package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"shell-guard/internal/api"
	"shell-guard/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	client := api.NewClient(cfg.SocketPath)
	ctx := context.Background()

	switch args[0] {
	case "daemon":
		if len(args) == 2 && args[1] == "run" {
			return fmt.Errorf("use shellguardd to run the daemon")
		}
		return fmt.Errorf("unknown daemon command")
	case "session":
		if len(args) < 2 {
			return fmt.Errorf("missing session subcommand")
		}
		switch args[1] {
		case "start":
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get cwd: %w", err)
			}
			return client.StartInteractiveSession(ctx, cfg.ShellPath, cwd)
		case "status":
			resp, err := client.Request(ctx, api.Request{Action: api.ActionSessionStatus})
			if err != nil {
				return err
			}
			if resp.Session == nil {
				fmt.Println("session: none")
				return nil
			}
			fmt.Printf("session: %s\n", resp.Session.Status)
			fmt.Printf("cwd: %s\n", resp.Session.CurrentCWD)
			fmt.Printf("shell: %s\n", resp.Session.ShellPath)
			fmt.Printf("pid: %d\n", resp.Session.ShellPID)
			return nil
		default:
			return fmt.Errorf("unknown session subcommand: %s", args[1])
		}
	case "state":
		resp, err := client.Request(ctx, api.Request{Action: api.ActionGetState})
		if err != nil {
			return err
		}
		if resp.State == nil {
			fmt.Println("state: unavailable")
			return nil
		}
		state := resp.State
		fmt.Printf("session: %s\n", state.SessionStatus)
		fmt.Printf("cwd: %s\n", blankIfEmpty(state.CurrentCWD))
		fmt.Printf("repo: %s\n", blankIfEmpty(state.RepoRoot))
		fmt.Printf("branch: %s\n", blankIfEmpty(state.GitBranch))
		fmt.Printf("dirty: %t\n", state.GitDirty)
		fmt.Printf("last_exit: %s\n", optionalInt(state.LastExitCode))
		fmt.Printf("last_summary: %s\n", blankIfEmpty(state.LastSummaryShort))
		return nil
	case "recent":
		limit := 10
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid recent limit: %w", err)
			}
			limit = n
		}
		resp, err := client.Request(ctx, api.Request{
			Action: api.ActionListRecentCommands,
			Limit:  limit,
		})
		if err != nil {
			return err
		}
		if len(resp.Recent) == 0 {
			fmt.Println("recent: none")
			return nil
		}
		for _, item := range resp.Recent {
			fmt.Printf("%d\t%s\texit=%d\t%s\t%s\n", item.ID, item.StartedAt.Format("2006-01-02 15:04:05"), item.ExitCode, item.RawCommand, item.SummaryShort)
		}
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func blankIfEmpty(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func optionalInt(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}

func printUsage() {
	fmt.Println("usage:")
	fmt.Println("  shg session start")
	fmt.Println("  shg session status")
	fmt.Println("  shg state")
	fmt.Println("  shg recent [limit]")
}
