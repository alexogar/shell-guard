package parsers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func resultWithJSON(summary string, payload map[string]any) Result {
	data, _ := json.Marshal(payload)
	return Result{SummaryShort: summary, StructuredSummary: string(data)}
}

func fallbackSummary(ctx Context) string {
	command := strings.TrimSpace(ctx.RawCommand)
	if command == "" {
		command = "command"
	}
	if len(command) > 60 {
		command = command[:57] + "..."
	}
	if ctx.ExitCode == 0 {
		return fmt.Sprintf("%s completed", command)
	}
	return fmt.Sprintf("%s failed with exit %d", command, ctx.ExitCode)
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func nonEmptyLines(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func lsEntries(output string) []string {
	seen := map[string]struct{}{}
	var entries []string
	for _, line := range nonEmptyLines(output) {
		for _, field := range strings.Fields(line) {
			if field == "%" || field == "shg$" {
				continue
			}
			if _, ok := seen[field]; ok {
				continue
			}
			seen[field] = struct{}{}
			entries = append(entries, field)
		}
	}
	return entries
}

func detectBranchFromStatus(output, fallback string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "On branch ") {
			return strings.TrimPrefix(line, "On branch ")
		}
	}
	return fallback
}

func countGitStatusSections(output string) (staged, unstaged, untracked int) {
	section := ""
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, "Changes to be committed:"):
			section = "staged"
		case strings.Contains(trimmed, "Changes not staged for commit:"):
			section = "unstaged"
		case strings.Contains(trimmed, "Untracked files:"):
			section = "untracked"
		case strings.HasPrefix(trimmed, "("):
			continue
		case strings.HasPrefix(line, "\t"):
			switch section {
			case "staged":
				staged++
			case "unstaged":
				unstaged++
			case "untracked":
				untracked++
			}
		}
	}
	return staged, unstaged, untracked
}

func detectPytestSummaryLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=") && strings.Contains(line, " in ") {
			trimmed := strings.Trim(line, "= ")
			if strings.Contains(trimmed, "passed") || strings.Contains(trimmed, "failed") || strings.Contains(trimmed, "error") {
				return trimmed
			}
		}
	}
	return ""
}

func detectPytestCount(line, label string) int {
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(strings.Trim(part, "="))
		if strings.HasSuffix(part, " "+label) {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				if n, err := strconv.Atoi(fields[0]); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func detectPytestDuration(line string) float64 {
	idx := strings.LastIndex(line, " in ")
	if idx == -1 {
		return 0
	}
	value := strings.TrimSpace(strings.TrimSuffix(line[idx+4:], "s"))
	duration, _ := strconv.ParseFloat(value, 64)
	return duration
}
