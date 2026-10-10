package history

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T, limit int) (*Store, string) {
	t.Helper()
	return &Store{Dir: filepath.Join(t.TempDir(), "history"), MaxEntries: limit}, filepath.Join(t.TempDir(), ".env")
}

func TestStoreRoundTrip(t *testing.T) {
	s, env := newTestStore(t, 10)

	got, err := s.Load(env)
	require.NoError(t, err)
	assert.Empty(t, got, "missing store is empty")

	require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: "1", Time: t0}}))
	require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: "2", Time: t0.Add(time.Minute)}}))

	got, err = s.Load(env)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, values(got))
	assert.Equal(t, SourceLocal, got[0].Source)
	assert.True(t, got[0].Time.Equal(t0))
}

func TestStoreTrimsPerKey(t *testing.T) {
	s, env := newTestStore(t, 2)

	for i, v := range []string{"1", "2", "3"} {
		require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: v, Time: t0.Add(time.Duration(i) * time.Minute)}}))
	}
	require.NoError(t, s.Record(env, []Entry{{Key: "B", Value: "b"}}))

	got, err := s.Load(env)
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3", "b"}, values(got))
}

func TestStoreFilesArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	s, env := newTestStore(t, 10)
	require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: "1"}}))

	files, err := filepath.Glob(filepath.Join(s.Dir, "*.jsonl"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	info, err := os.Stat(files[0])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	dirInfo, err := os.Stat(s.Dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

func TestStoreSeparatesFiles(t *testing.T) {
	s, env := newTestStore(t, 10)
	other := filepath.Join(filepath.Dir(env), ".env.local")
	require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: "1"}}))

	got, err := s.Load(other)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStoreSkipsCorruptLines(t *testing.T) {
	s, env := newTestStore(t, 10)
	require.NoError(t, s.Record(env, []Entry{{Key: "A", Value: "1"}}))
	abs, err := filepath.Abs(env)
	require.NoError(t, err)
	f, err := os.OpenFile(s.file(abs), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString("not json\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	got, err := s.Load(env)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, values(got))
}

func TestDefaultDirXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	dir, err := DefaultDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(xdg, "lazyenv", "history"), dir)
}

func TestDefaultDirPlatform(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	var want string
	switch runtime.GOOS {
	case "windows":
		base, err := os.UserCacheDir()
		require.NoError(t, err)
		want = base
	case "darwin":
		base, err := os.UserConfigDir()
		require.NoError(t, err)
		want = base
	default:
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		want = filepath.Join(home, ".local", "state")
	}

	dir, err := DefaultDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(want, "lazyenv", "history"), dir)
}
