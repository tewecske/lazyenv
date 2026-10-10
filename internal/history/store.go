package history

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lazynop/lazyenv/internal/parser"
)

// storePerm keeps history files private: they may hold secret values.
const (
	storePerm    os.FileMode = 0o600
	storeDirPerm os.FileMode = 0o700
)

// Store keeps local history as one JSON-lines file per env file.
type Store struct {
	Dir        string
	MaxEntries int // entries kept per key
}

// DefaultDir returns $XDG_STATE_HOME/lazyenv/history if set, else the
// platform's per-user app data dir (see stateBase).
func DefaultDir() (string, error) {
	base, err := stateBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lazyenv", "history"), nil
}

// stateBase is %LocalAppData% on Windows, ~/Library/Application Support on
// macOS and ~/.local/state elsewhere.
func stateBase() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return xdg, nil
	}
	var (
		dir string
		err error
	)
	switch runtime.GOOS {
	case "windows":
		dir, err = os.UserCacheDir()
	case "darwin":
		dir, err = os.UserConfigDir()
	default:
		dir, err = os.UserHomeDir()
		dir = filepath.Join(dir, ".local", "state")
	}
	if err != nil {
		return "", fmt.Errorf("finding state directory: %w", err)
	}
	return dir, nil
}

// record is one line of a store file. Path guards against hash collisions.
type record struct {
	Path string `json:"path"`
	Entry
}

// Load returns the local entries for the env file at envPath, oldest first.
func (s *Store) Load(envPath string) ([]Entry, error) {
	abs, err := filepath.Abs(envPath)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", envPath, err)
	}
	data, err := os.ReadFile(s.file(abs))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading history: %w", err)
	}
	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var r record
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.Path != abs {
			continue // skip corrupt or foreign lines
		}
		r.Source = SourceLocal
		out = append(out, r.Entry)
	}
	return out, sc.Err()
}

// Record appends entries for envPath, keeping the newest MaxEntries per key.
func (s *Store) Record(envPath string, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	abs, err := filepath.Abs(envPath)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", envPath, err)
	}
	existing, err := s.Load(abs)
	if err != nil {
		return err
	}
	all := trimPerKey(append(existing, entries...), s.MaxEntries)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, e := range all {
		if err := enc.Encode(record{Path: abs, Entry: e}); err != nil {
			return fmt.Errorf("encoding history: %w", err)
		}
	}
	if err := os.MkdirAll(s.Dir, storeDirPerm); err != nil {
		return fmt.Errorf("creating history dir: %w", err)
	}
	return parser.WriteFileAtomic(s.file(abs), buf.Bytes(), storePerm)
}

// file returns the store file for an absolute env path.
func (s *Store) file(abs string) string {
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(s.Dir, hex.EncodeToString(sum[:8])+".jsonl")
}

// trimPerKey keeps the last limit entries of each key, preserving order.
func trimPerKey(entries []Entry, limit int) []Entry {
	if limit <= 0 {
		return entries
	}
	count := make(map[string]int)
	for _, e := range entries {
		count[e.Key]++
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if count[e.Key] > limit {
			count[e.Key]--
			continue
		}
		out = append(out, e)
	}
	return out
}
