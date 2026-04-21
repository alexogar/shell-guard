package state

import "testing"

func TestSummarizeCommand(t *testing.T) {
	if got := SummarizeCommand("pwd", 0); got != "pwd completed" {
		t.Fatalf("unexpected success summary: %q", got)
	}
	if got := SummarizeCommand("pytest", 2); got != "pytest failed with exit 2" {
		t.Fatalf("unexpected failure summary: %q", got)
	}
}

func TestGuessCommandFamily(t *testing.T) {
	if got := GuessCommandFamily("git status"); got != "git" {
		t.Fatalf("unexpected command family: %q", got)
	}
	if got := GuessCommandFamily(""); got != "" {
		t.Fatalf("expected empty family, got %q", got)
	}
}
