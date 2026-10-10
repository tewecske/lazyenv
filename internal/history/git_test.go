package history

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lazynop/lazyenv/internal/config"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	run(t, dir, "config", "user.name", "Tester")
	run(t, dir, "config", "user.email", "t@example.com")
	run(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func commitFile(t *testing.T, dir, name, content, date string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	run(t, dir, "add", name)
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	run(t, dir, "commit", "-q", "-m", "update "+name)
}

func TestGitHistory(t *testing.T) {
	dir := gitRepo(t)
	commitFile(t, dir, ".env.example", "A=1\nB=x\n", "2026-01-01T00:00:00Z")
	commitFile(t, dir, ".env.example", "A=2\nB=x\n", "2026-02-01T00:00:00Z")
	commitFile(t, dir, "other", "unrelated", "2026-02-15T00:00:00Z")
	commitFile(t, dir, ".env.example", "A=3\n", "2026-03-01T00:00:00Z")
	commitFile(t, dir, ".env.example", "A=3\nB=y\n", "2026-04-01T00:00:00Z")

	entries, err := Git(filepath.Join(dir, ".env.example"), 50, config.SecretsConfig{})
	require.NoError(t, err)

	a := ForKey(entries, "A", "")
	assert.Equal(t, []string{"3", "2", "1"}, values(a))
	assert.Equal(t, "Tester", a[0].Author)
	assert.Len(t, a[0].Source, 7)
	assert.Equal(t, 2026, a[0].Time.Year())

	assert.Equal(t, []string{"y", "x"}, values(ForKey(entries, "B", "")), "B re-added after deletion")
}

func TestGitHistoryMaxCommits(t *testing.T) {
	dir := gitRepo(t)
	commitFile(t, dir, ".env", "A=1\n", "2026-01-01T00:00:00Z")
	commitFile(t, dir, ".env", "A=2\n", "2026-02-01T00:00:00Z")
	commitFile(t, dir, ".env", "A=3\n", "2026-03-01T00:00:00Z")

	entries, err := Git(filepath.Join(dir, ".env"), 2, config.SecretsConfig{})
	require.NoError(t, err)
	assert.Equal(t, []string{"3", "2"}, values(ForKey(entries, "A", "")))
}

func TestGitHistoryUntracked(t *testing.T) {
	dir := gitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("A=1\n"), 0o644))

	entries, err := Git(filepath.Join(dir, ".env"), 50, config.SecretsConfig{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestGitHistoryNotARepo(t *testing.T) {
	entries, err := Git(filepath.Join(t.TempDir(), ".env"), 50, config.SecretsConfig{})
	require.NoError(t, err)
	assert.Empty(t, entries)
}
