package parsers

import "fmt"

type pwdParser struct{}

func (pwdParser) Name() string { return "pwd" }

func (pwdParser) Match(ctx Context) bool {
	return ctx.CommandName == "pwd"
}

func (pwdParser) Parse(ctx Context) Result {
	path := firstNonEmptyLine(ctx.Output)
	if path == "" {
		path = ctx.CWD
	}
	return resultWithJSON(fmt.Sprintf("cwd is %s", path), map[string]any{"path": path})
}
