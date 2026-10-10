package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lazynop/lazyenv/internal/history"
)

// newHistoryTestApp returns an app on a disk-backed .env with git history off.
func newHistoryTestApp(t *testing.T, content string) App {
	t.Helper()
	app, _ := newTestAppWithDiskFile(t, content)
	app.history = &history.Store{Dir: t.TempDir(), MaxEntries: 20}
	app.config.History.Git = false
	return app
}

func editAndSave(t *testing.T, app App, idx int, value string) App {
	t.Helper()
	app.varList.File.UpdateVar(idx, value)
	updated, _ := app.handleSave()
	return updated
}

func TestSaveRecordsReplacedValues(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\nB=2\n")

	app = editAndSave(t, app, 0, "10")

	entries, err := app.history.Load(app.varList.File.Path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "A", entries[0].Key)
	assert.Equal(t, "1", entries[0].Value)
}

func TestSaveRecordsDeletedVar(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\nB=2\n")
	app.varList.File.DeleteVar(1)
	app, _ = app.handleSave()

	entries, err := app.history.Load(app.varList.File.Path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "B", entries[0].Key)
}

func TestSaveSkipsSecretsByDefault(t *testing.T) {
	app := newHistoryTestApp(t, "API_TOKEN=abc\n")
	app = editAndSave(t, app, 0, "def")

	entries, err := app.history.Load(app.varList.File.Path)
	require.NoError(t, err)
	assert.Empty(t, entries)

	app.config.History.RecordSecrets = true
	app = editAndSave(t, app, 0, "ghi")
	entries, err = app.history.Load(app.varList.File.Path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Secret)
}

func TestSaveWithHistoryDisabled(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app.history = nil

	app = editAndSave(t, app, 0, "2")
	assert.Contains(t, app.statusBar.Message, "Saved")
}

func TestHistoryOpenRestoreClose(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app = editAndSave(t, app, 0, "2")
	app = editAndSave(t, app, 0, "3")

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})
	require.Equal(t, ModeHistory, app.mode)
	assert.Equal(t, []string{"2", "1"}, historyValues(app.histView.Entries))

	app = pressKey(app, tea.KeyPressMsg{Text: "j"})
	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, ModeNormal, app.mode)
	v := app.varList.SelectedVar()
	assert.Equal(t, "1", v.Value)
	assert.True(t, v.Modified, "restore goes through the normal edit flow")
	assert.Contains(t, app.statusBar.Message, "Restored A")

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})
	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, ModeNormal, app.mode)
}

func TestHistoryEmpty(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})

	assert.Equal(t, ModeHistory, app.mode)
	assert.Contains(t, app.View().Content, "No previous values")
	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, ModeHistory, app.mode, "enter on an empty list does nothing")
}

func TestHistoryDisabled(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app.history = nil

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})

	assert.Equal(t, ModeNormal, app.mode)
	assert.Contains(t, app.statusBar.Message, "disabled")
}

func TestHistoryNoOpOnFilePanel(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app.focus = FocusFiles

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})

	assert.Equal(t, ModeNormal, app.mode)
}

func TestHistoryRestoreBlockedInReadOnly(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app = editAndSave(t, app, 0, "2")
	app.config.ReadOnly = true

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})
	require.Equal(t, ModeHistory, app.mode, "viewing works in read-only")
	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Equal(t, "2", app.varList.SelectedVar().Value)
	assert.False(t, app.varList.File.Modified)
}

func TestHistoryGitLoadsAsyncAndCaches(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app.config.History.Git = true
	path := app.varList.File.Path

	updated, cmd := app.Update(tea.KeyPressMsg{Text: "H"})
	app = updated.(App)
	require.NotNil(t, cmd, "git history loads in the background")
	assert.True(t, app.histView.Loading)
	assert.Contains(t, app.View().Content, "Loading git history")

	git := []history.Entry{{Key: "A", Value: "from-git", Time: time.Now(), Source: "abc1234", Author: "Tester"}}
	updated, _ = app.Update(GitHistoryMsg{Path: path, Entries: git})
	app = updated.(App)
	assert.False(t, app.histView.Loading)
	assert.Equal(t, []string{"from-git"}, historyValues(app.histView.Entries))
	assert.Contains(t, app.View().Content, "abc1234 Tester")

	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEscape})
	updated, cmd = app.Update(tea.KeyPressMsg{Text: "H"})
	app = updated.(App)
	assert.Nil(t, cmd, "second open uses the cache")
	assert.Equal(t, []string{"from-git"}, historyValues(app.histView.Entries))
}

func TestHistoryGitError(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	app.config.History.Git = true
	app = pressKey(app, tea.KeyPressMsg{Text: "H"})

	updated, _ := app.Update(GitHistoryMsg{Path: app.varList.File.Path, Err: errors.New("boom")})
	app = updated.(App)

	assert.Contains(t, app.View().Content, "boom")
	assert.NotContains(t, app.gitHistory, app.varList.File.Path, "errors are not cached")
}

func TestHistoryMasksSecrets(t *testing.T) {
	app := newHistoryTestApp(t, "API_TOKEN=abc\n")
	app.config.History.RecordSecrets = true
	app = editAndSave(t, app, 0, "supersecretvalue")

	app = pressKey(app, tea.KeyPressMsg{Text: "H"})
	assert.NotContains(t, app.View().Content, "abc")

	app = pressKey(app, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Contains(t, app.View().Content, "abc")
}

func TestHistoryMouse(t *testing.T) {
	app := newHistoryTestApp(t, "A=1\n")
	for _, v := range []string{"2", "3", "4"} {
		app = editAndSave(t, app, 0, v)
	}
	app = pressKey(app, tea.KeyPressMsg{Text: "H"})
	require.Len(t, app.histView.Entries, 3)

	updated, _ := app.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	app = updated.(App)
	assert.Equal(t, 1, app.histView.Cursor)

	popup := app.histView.View(app.theme, app.width, app.height, false)
	x, y := app.popupOrigin(popup)
	updated, _ = app.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x + 3, Y: y + historyListOffsetY + 2})
	app = updated.(App)
	assert.Equal(t, 2, app.histView.Cursor, "click selects the row")
	assert.Equal(t, ModeHistory, app.mode)

	updated, _ = app.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
	app = updated.(App)
	assert.Equal(t, ModeNormal, app.mode, "click outside closes")
}

func TestHistoryViewScrolls(t *testing.T) {
	var entries []history.Entry
	for i := range 30 {
		entries = append(entries, history.Entry{Key: "A", Value: string(rune('a' + i%26)), Time: time.Unix(int64(i), 0)})
	}
	m := NewHistoryView("p", "A", "cur", entries, 20)

	for range 29 {
		m.MoveDown()
	}

	assert.Equal(t, 29, m.Cursor)
	assert.Equal(t, 29-m.visibleRows()+1, m.Offset)
	m.SetCursor(-5)
	assert.Equal(t, 0, m.Cursor)
	assert.Equal(t, 0, m.Offset)
}

func historyValues(entries []history.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Value
	}
	return out
}
