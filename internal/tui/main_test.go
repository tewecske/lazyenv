package tui

import (
	"os"
	"testing"
)

// TestMain keeps value-history writes from tests out of the real state dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lazyenv-state-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
