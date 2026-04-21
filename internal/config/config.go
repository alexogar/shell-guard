package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultInlineLimit = 64 * 1024

type Config struct {
	HomeDir           string
	SocketPath        string
	DBPath            string
	OutputDir         string
	ShellPath         string
	InlineOutputLimit int64
}

func Load() (Config, error) {
	homeDir, err := resolveHome()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HomeDir:           homeDir,
		SocketPath:        envOrDefault("SHG_SOCKET", filepath.Join(homeDir, "shellguard.sock")),
		DBPath:            envOrDefault("SHG_DB", filepath.Join(homeDir, "shellguard.db")),
		OutputDir:         envOrDefault("SHG_OUTPUT_DIR", filepath.Join(homeDir, "outputs")),
		ShellPath:         resolveShell(),
		InlineOutputLimit: defaultInlineLimit,
	}

	if v := os.Getenv("SHG_INLINE_OUTPUT_LIMIT"); v != "" {
		limit, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse SHG_INLINE_OUTPUT_LIMIT: %w", err)
		}
		cfg.InlineOutputLimit = limit
	}

	if err := validateShell(cfg.ShellPath); err != nil {
		return Config{}, err
	}

	if err := os.MkdirAll(cfg.HomeDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create home dir: %w", err)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create output dir: %w", err)
	}

	return cfg, nil
}

func resolveHome() (string, error) {
	if v := os.Getenv("SHG_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".shellguard"), nil
}

func resolveShell() string {
	if v := os.Getenv("SHG_SHELL"); v != "" {
		return v
	}
	if v := os.Getenv("SHELL"); v != "" {
		return v
	}
	return "/bin/zsh"
}

func validateShell(shell string) error {
	base := filepath.Base(shell)
	switch strings.ToLower(base) {
	case "bash", "zsh":
		return nil
	default:
		return fmt.Errorf("unsupported shell %q: only bash and zsh are supported in the MVP", shell)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
