package parsers

import (
	"fmt"
	"path/filepath"
)

type lsParser struct{}

func (lsParser) Name() string { return "ls" }

func (lsParser) Match(ctx Context) bool {
	return ctx.CommandName == "ls"
}

func (lsParser) Parse(ctx Context) Result {
	entries := lsEntries(ctx.Output)
	count := len(entries)
	summary := "directory listing empty"
	if count == 1 {
		summary = fmt.Sprintf("listed 1 entry in %s", filepath.Base(ctx.CWD))
	} else if count > 1 {
		summary = fmt.Sprintf("listed %d entries in %s", count, filepath.Base(ctx.CWD))
	}
	preview := entries
	if len(preview) > 5 {
		preview = preview[:5]
	}
	return resultWithJSON(summary, map[string]any{"count": count, "preview": preview})
}
