package session

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkerLineBegin(t *testing.T) {
	cwd := "/tmp/project"
	command := "git status"
	line := "__SHG__|BEGIN|" + b64(cwd) + "|" + b64(command)

	got, ok := parseMarkerLine(line)
	if !ok {
		t.Fatal("expected begin marker to parse")
	}
	if got.Kind != "BEGIN" || got.CWD != cwd || got.Command != command {
		t.Fatalf("unexpected marker: %+v", got)
	}
}

func TestParseMarkerLineEnd(t *testing.T) {
	cwd := "/tmp/project"
	line := "__SHG__|END|7|" + b64(cwd)

	got, ok := parseMarkerLine(line)
	if !ok {
		t.Fatal("expected end marker to parse")
	}
	if got.Kind != "END" || got.CWD != cwd || got.ExitCode != 7 {
		t.Fatalf("unexpected marker: %+v", got)
	}
}

func TestParseMarkerLineRejectsInvalidData(t *testing.T) {
	if _, ok := parseMarkerLine("__SHG__|BEGIN|not-base64|also-bad"); ok {
		t.Fatal("expected invalid marker to be rejected")
	}
}

func TestSplitCommand(t *testing.T) {
	name, args := splitCommand("git branch --show-current")
	if name != "git" {
		t.Fatalf("unexpected name: %q", name)
	}
	if strings.Join(args, ",") != "branch,--show-current" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestNormalizeParserOutput(t *testing.T) {
	raw := "\x1b[?2004hls\x1b[?2004l\r\nREADME.md\tcmd\r\n%\t\t\t\t\r\nshg$ \r\n"
	got := normalizeParserOutput(raw)
	want := "ls\nREADME.md\tcmd"
	if got != want {
		t.Fatalf("unexpected normalized output:\nwant %q\ngot  %q", want, got)
	}
}

func TestRepoRootOrCWD(t *testing.T) {
	if got := repoRootOrCWD("/tmp/repo", "/tmp/repo/subdir"); got != "/tmp/repo" {
		t.Fatalf("unexpected repo root preference: %q", got)
	}
	if got := repoRootOrCWD("", "/tmp/repo/subdir"); got != "/tmp/repo/subdir" {
		t.Fatalf("unexpected cwd fallback: %q", got)
	}
}

func TestPrepareShellCommandRejectsUnsupportedShell(t *testing.T) {
	_, err := prepareShellCommand("/bin/fish", t.TempDir(), filepath.Join(t.TempDir(), "shell"))
	if err == nil {
		t.Fatal("expected unsupported shell to fail")
	}
}

func b64(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}
