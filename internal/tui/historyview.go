package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lazynop/lazyenv/internal/history"
	"github.com/lazynop/lazyenv/internal/util"
)

// HistoryViewModel is the popup listing past values of one variable.
type HistoryViewModel struct {
	Path    string
	Key     string
	Current string
	Entries []history.Entry // newest first, current value excluded
	Cursor  int
	Offset  int
	Height  int // terminal height, for scrolling
	Loading bool
	Err     string

	local []history.Entry
}

// NewHistoryView builds the popup from the local entries of a file.
func NewHistoryView(path, key, current string, local []history.Entry, height int) HistoryViewModel {
	m := HistoryViewModel{Path: path, Key: key, Current: current, Height: height, local: local}
	m.Entries = history.ForKey(local, key, current)
	return m
}

// SetGit merges git entries into the list.
func (m *HistoryViewModel) SetGit(git []history.Entry) {
	m.Entries = history.ForKey(append(append([]history.Entry{}, m.local...), git...), m.Key, m.Current)
	m.SetCursor(m.Cursor)
}

// Selected returns the entry under the cursor, or nil.
func (m *HistoryViewModel) Selected() *history.Entry {
	if m.Cursor < 0 || m.Cursor >= len(m.Entries) {
		return nil
	}
	return &m.Entries[m.Cursor]
}

func (m *HistoryViewModel) MoveUp()   { m.SetCursor(m.Cursor - 1) }
func (m *HistoryViewModel) MoveDown() { m.SetCursor(m.Cursor + 1) }

// SetCursor moves the cursor to index, clamped, and keeps it visible.
func (m *HistoryViewModel) SetCursor(index int) {
	m.Cursor = max(0, min(index, len(m.Entries)-1))
	rows := m.visibleRows()
	if m.Cursor < m.Offset {
		m.Offset = m.Cursor
	}
	if m.Cursor >= m.Offset+rows {
		m.Offset = m.Cursor - rows + 1
	}
}

func (m *HistoryViewModel) visibleRows() int {
	return max(1, m.Height-popupChromeHeight-historyFixedRows-popupMarginRows)
}

// View renders the popup box.
func (m HistoryViewModel) View(theme Theme, width, height int, showSecrets bool) string {
	m.Height = height
	inner := max(1, min(historyMaxWidth, width)-popupChromeWidth)

	lines := []string{theme.KeyStyle.Render("History: " + m.Key), ""}
	switch {
	case m.Err != "":
		lines = append(lines, theme.DuplicateWarn.Render(truncate(m.Err, inner)))
	case len(m.Entries) == 0 && m.Loading:
		lines = append(lines, theme.MutedItem.Render("Loading git history…"))
	case len(m.Entries) == 0:
		lines = append(lines, theme.MutedItem.Render("No previous values"))
	}
	end := min(len(m.Entries), m.Offset+m.visibleRows())
	metaWidth := 0
	for _, e := range m.Entries[m.Offset:end] {
		metaWidth = max(metaWidth, lipgloss.Width(entryMeta(e)))
	}
	metaWidth = min(metaWidth, inner/2)
	for i := m.Offset; i < end; i++ {
		lines = append(lines, m.row(theme, m.Entries[i], i == m.Cursor, inner, metaWidth, showSecrets))
	}
	if m.Loading && len(m.Entries) > 0 {
		lines = append(lines, theme.MutedItem.Render("Loading git history…"))
	}
	lines = append(lines, "", theme.MutedItem.Render("enter restore · ^S secrets · esc close"))

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimary).
		Padding(1, 2)
	return box.Render(strings.Join(lines, "\n"))
}

// entryMeta describes where an entry came from: time, source and author.
func entryMeta(e history.Entry) string {
	meta := e.Time.Local().Format("2006-01-02 15:04") + "  " + e.Source
	if e.Author != "" {
		meta += " " + e.Author
	}
	return meta
}

// row renders one entry: cursor marker, value, then its meta column.
func (m HistoryViewModel) row(theme Theme, e history.Entry, cursor bool, width, metaWidth int, showSecrets bool) string {
	value := flattenValue(e.Value)
	if e.Secret && !showSecrets {
		value = util.MaskValue(value)
	}
	if value == "" {
		value = "(empty)"
	}
	meta := truncate(entryMeta(e), metaWidth)
	valueWidth := max(1, width-2-metaWidth-1)
	value = fmt.Sprintf("%-*s", valueWidth, truncate(value, valueWidth))

	marker, style := "  ", theme.NormalItem
	if cursor {
		marker, style = "▸ ", theme.CursorItem
	}
	return style.Render(marker+value) + " " + theme.MutedItem.Render(meta)
}
