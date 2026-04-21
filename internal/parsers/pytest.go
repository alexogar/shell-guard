package parsers

type pytestParser struct{}

func (pytestParser) Name() string { return "pytest" }

func (pytestParser) Match(ctx Context) bool {
	return ctx.CommandName == "pytest"
}

func (pytestParser) Parse(ctx Context) Result {
	summaryLine := detectPytestSummaryLine(ctx.Output)
	summary := fallbackSummary(ctx)
	if summaryLine != "" {
		summary = summaryLine
	}
	return resultWithJSON(summary, map[string]any{
		"passed":     detectPytestCount(summaryLine, "passed"),
		"failed":     detectPytestCount(summaryLine, "failed"),
		"errors":     detectPytestCount(summaryLine, "error"),
		"skipped":    detectPytestCount(summaryLine, "skipped"),
		"duration_s": detectPytestDuration(summaryLine),
	})
}
