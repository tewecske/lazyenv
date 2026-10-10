package tui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/lazynop/lazyenv/internal/history"
	"github.com/lazynop/lazyenv/internal/model"
)

// historyBaseline returns the vars on disk before f is saved, or nil when
// history is off or the file cannot be read.
func (a App) historyBaseline(f *model.EnvFile) []model.EnvVar {
	if a.history == nil {
		return nil
	}
	disk, err := reparse(f, a.config.Secrets)
	if err != nil {
		return nil
	}
	return disk.Vars
}

// recordHistory stores the values a save replaced and returns a warning on failure.
func (a App) recordHistory(path string, before, after []model.EnvVar) string {
	if a.history == nil {
		return ""
	}
	replaced := history.Replaced(before, after, a.config.History.RecordSecrets, time.Now())
	if err := a.history.Record(path, replaced); err != nil {
		return "history failed: " + err.Error() + " - "
	}
	return ""
}

// gitHistoryMaxCommits bounds how far back git history is read.
const gitHistoryMaxCommits = 50

// GitHistoryMsg carries the git history of one file, read in the background.
type GitHistoryMsg struct {
	Path    string
	Entries []history.Entry
	Err     error
}

// openHistory shows the past values of the selected variable.
func (a App) openHistory() (tea.Model, tea.Cmd) {
	if a.history == nil {
		return a, a.flashMessage("Value history is disabled")
	}
	v := a.varList.SelectedVar()
	if v == nil || a.varList.IsHeaderAtCursor() {
		return a, a.flashMessage("No variable selected")
	}
	f := a.varList.File
	local, err := a.history.Load(f.Path)
	if err != nil {
		return a, a.flashError("Error reading history: " + err.Error())
	}
	a.histView = NewHistoryView(f.Path, v.Key, v.Value, local, a.height)
	a.mode = ModeHistory

	if !a.config.History.Git {
		return a, nil
	}
	if cached, ok := a.gitHistory[f.Path]; ok {
		a.histView.SetGit(cached)
		return a, nil
	}
	a.histView.Loading = true
	path, secrets := f.Path, a.config.Secrets
	return a, func() tea.Msg {
		entries, err := history.Git(path, gitHistoryMaxCommits, secrets)
		return GitHistoryMsg{Path: path, Entries: entries, Err: err}
	}
}

// handleGitHistory caches a git result and fills the open popup.
func (a App) handleGitHistory(msg GitHistoryMsg) App {
	if msg.Err == nil {
		a.gitHistory[msg.Path] = msg.Entries
	}
	if a.mode != ModeHistory || a.histView.Path != msg.Path {
		return a
	}
	a.histView.Loading = false
	if msg.Err != nil {
		a.histView.Err = msg.Err.Error()
	}
	a.histView.SetGit(msg.Entries)
	return a
}

func (a App) handleHistoryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Escape, a.keys.History, a.keys.Quit):
		a.mode = ModeNormal
	case key.Matches(msg, a.keys.Up):
		a.histView.MoveUp()
	case key.Matches(msg, a.keys.Down):
		a.histView.MoveDown()
	case key.Matches(msg, a.keys.ToggleSecret):
		a.varList.ShowSecrets = !a.varList.ShowSecrets
	case key.Matches(msg, a.keys.Enter):
		return a.restoreHistory()
	}
	return a, nil
}

// restoreHistory sets the variable to the selected past value as a normal edit.
func (a App) restoreHistory() (tea.Model, tea.Cmd) {
	if cmd := a.readOnlyFlash(); cmd != nil {
		return a, cmd
	}
	e := a.histView.Selected()
	idx := a.varList.SelectedVarIndex()
	if e == nil || idx < 0 || a.varList.File.Vars[idx].Key != a.histView.Key {
		return a, nil
	}
	a.varList.File.UpdateVar(idx, e.Value)
	a.varList.Refresh()
	a.mode = ModeNormal
	return a, a.flashMessage("Restored " + e.Key + " from " + e.Source)
}

func (a App) scrollHistory(up bool) App {
	for range a.config.Layout.MouseScrollLines {
		if up {
			a.histView.MoveUp()
		} else {
			a.histView.MoveDown()
		}
	}
	return a
}

// handleHistoryClick selects a clicked row; a click outside the popup closes it.
func (a App) handleHistoryClick(msg tea.MouseClickMsg) App {
	popup := a.histView.View(a.theme, a.width, a.height, a.varList.ShowSecrets)
	x, y := a.popupOrigin(popup)
	w, h := popupSize(popup)
	if msg.X < x || msg.X >= x+w || msg.Y < y || msg.Y >= y+h {
		a.mode = ModeNormal
		return a
	}
	row := msg.Y - y - historyListOffsetY
	if row >= 0 && row < a.histView.visibleRows() && a.histView.Offset+row < len(a.histView.Entries) {
		a.histView.SetCursor(a.histView.Offset + row)
	}
	return a
}
