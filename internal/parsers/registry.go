package parsers

type Context struct {
	RawCommand  string
	CommandName string
	Args        []string
	CWD         string
	RepoRoot    string
	GitBranch   string
	GitDirty    bool
	ExitCode    int
	Output      string
	DurationMS  int64
}

type Result struct {
	Name              string
	SummaryShort      string
	StructuredSummary string
}

type Parser interface {
	Name() string
	Match(ctx Context) bool
	Parse(ctx Context) Result
}

type Registry struct {
	parsers []Parser
}

func DefaultRegistry() *Registry {
	return &Registry{
		parsers: []Parser{
			pwdParser{},
			gitBranchShowCurrentParser{},
			gitStatusParser{},
			lsParser{},
			pytestParser{},
		},
	}
}

func (r *Registry) Parse(ctx Context) (Result, bool) {
	for _, parser := range r.parsers {
		if parser.Match(ctx) {
			result := parser.Parse(ctx)
			result.Name = parser.Name()
			if result.SummaryShort == "" {
				result.SummaryShort = fallbackSummary(ctx)
			}
			return result, true
		}
	}
	return Result{}, false
}
