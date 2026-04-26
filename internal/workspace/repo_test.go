package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootReturnsGitDirectoryFromNestedPath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}

	got, err := FindRepoRoot(nested)
	if err != nil {
		t.Fatalf("FindRepoRoot returned error: %v", err)
	}
	if got != root {
		t.Fatalf("FindRepoRoot() = %q, want %q", got, root)
	}
}

func TestFindRepoRootReturnsErrorOutsideGitRepository(t *testing.T) {
	_, err := FindRepoRoot(t.TempDir())
	if err == nil {
		t.Fatal("FindRepoRoot() error = nil, want error")
	}
}

func TestFindRepoRootAcceptsGitFileForWorktrees(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src")
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../actual/.git\n"), 0o644); err != nil {
		t.Fatalf("writing .git file: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}

	got, err := FindRepoRoot(nested)
	if err != nil {
		t.Fatalf("FindRepoRoot returned error: %v", err)
	}
	if got != root {
		t.Fatalf("FindRepoRoot() = %q, want %q", got, root)
	}
}
