package state

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SummarizeCommand(raw string, exitCode int) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = "command"
	}
	if len(name) > 60 {
		name = name[:57] + "..."
	}
	if exitCode == 0 {
		return fmt.Sprintf("%s completed", name)
	}
	return fmt.Sprintf("%s failed with exit %d", name, exitCode)
}

func GuessCommandFamily(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
