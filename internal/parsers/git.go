package parsers

import (
	"fmt"
	"strings"
)

type gitBranchShowCurrentParser struct{}

func (gitBranchShowCurrentParser) Name() string { return "git_branch_show_current" }

func (gitBranchShowCurrentParser) Match(ctx Context) bool {
	return ctx.CommandName == "git" && len(ctx.Args) >= 2 && ctx.Args[0] == "branch" && ctx.Args[1] == "--show-current"
}

func (gitBranchShowCurrentParser) Parse(ctx Context) Result {
	branch := firstNonEmptyLine(ctx.Output)
	if branch == "" {
		branch = ctx.GitBranch
	}
	summary := "git branch unavailable"
	if branch != "" {
		summary = fmt.Sprintf("current branch is %s", branch)
	}
	return resultWithJSON(summary, map[string]any{"branch": branch})
}

type gitStatusParser struct{}

func (gitStatusParser) Name() string { return "git_status" }

func (gitStatusParser) Match(ctx Context) bool {
	return ctx.CommandName == "git" && len(ctx.Args) >= 1 && ctx.Args[0] == "status"
}

func (gitStatusParser) Parse(ctx Context) Result {
	output := ctx.Output
	if strings.Contains(output, "nothing to commit, working tree clean") {
		return resultWithJSON("git working tree is clean", map[string]any{
			"dirty":     false,
			"branch":    detectBranchFromStatus(output, ctx.GitBranch),
			"staged":    0,
			"unstaged":  0,
			"untracked": 0,
		})
	}

	staged, unstaged, untracked := countGitStatusSections(output)
	branch := detectBranchFromStatus(output, ctx.GitBranch)
	dirty := staged+unstaged+untracked > 0 || ctx.GitDirty
	parts := []string{}
	if staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", staged))
	}
	if unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", unstaged))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", untracked))
	}
	summary := "git status captured"
	if dirty && len(parts) > 0 {
		summary = fmt.Sprintf("git working tree dirty: %s", strings.Join(parts, ", "))
	}
	return resultWithJSON(summary, map[string]any{
		"dirty":     dirty,
		"branch":    branch,
		"staged":    staged,
		"unstaged":  unstaged,
		"untracked": untracked,
	})
}
