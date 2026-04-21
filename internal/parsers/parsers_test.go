package parsers

import "testing"

func TestPWDParser(t *testing.T) {
	reg := DefaultRegistry()
	result, ok := reg.Parse(Context{
		RawCommand:  "pwd",
		CommandName: "pwd",
		CWD:         "/tmp/project",
		ExitCode:    0,
		Output:      "/tmp/project\n",
	})
	if !ok {
		t.Fatal("expected pwd parser to match")
	}
	if result.SummaryShort != "cwd is /tmp/project" {
		t.Fatalf("unexpected summary: %q", result.SummaryShort)
	}
}

func TestGitBranchParser(t *testing.T) {
	reg := DefaultRegistry()
	result, ok := reg.Parse(Context{
		RawCommand:  "git branch --show-current",
		CommandName: "git",
		Args:        []string{"branch", "--show-current"},
		ExitCode:    0,
		Output:      "main\n",
	})
	if !ok {
		t.Fatal("expected git branch parser to match")
	}
	if result.SummaryShort != "current branch is main" {
		t.Fatalf("unexpected summary: %q", result.SummaryShort)
	}
}

func TestGitStatusParserClean(t *testing.T) {
	reg := DefaultRegistry()
	result, ok := reg.Parse(Context{
		RawCommand:  "git status",
		CommandName: "git",
		Args:        []string{"status"},
		GitBranch:   "main",
		ExitCode:    0,
		Output:      "On branch main\nnothing to commit, working tree clean\n",
	})
	if !ok {
		t.Fatal("expected git status parser to match")
	}
	if result.SummaryShort != "git working tree is clean" {
		t.Fatalf("unexpected summary: %q", result.SummaryShort)
	}
}

func TestLSParser(t *testing.T) {
	reg := DefaultRegistry()
	result, ok := reg.Parse(Context{
		RawCommand:  "ls",
		CommandName: "ls",
		CWD:         "/tmp/shell-guard",
		ExitCode:    0,
		Output: "README.md\tcmd\t\tgo.sum\t\tmigrations\tshg\n" +
			"SPEC.md\t\tgo.mod\t\tinternal\tshellguardd\n" +
			"%                                                                              \n",
	})
	if !ok {
		t.Fatal("expected ls parser to match")
	}
	if result.SummaryShort != "listed 9 entries in shell-guard" {
		t.Fatalf("unexpected summary: %q", result.SummaryShort)
	}
}

func TestPytestParser(t *testing.T) {
	reg := DefaultRegistry()
	result, ok := reg.Parse(Context{
		RawCommand:  "pytest -q",
		CommandName: "pytest",
		Args:        []string{"-q"},
		ExitCode:    1,
		Output:      "======================= 2 failed, 18 passed in 1.23s =======================\n",
	})
	if !ok {
		t.Fatal("expected pytest parser to match")
	}
	if result.SummaryShort != "2 failed, 18 passed in 1.23s" {
		t.Fatalf("unexpected summary: %q", result.SummaryShort)
	}
}
