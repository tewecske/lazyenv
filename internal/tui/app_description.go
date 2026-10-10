package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// openDescription shows the selected variable's comments in a popup.
func (a App) openDescription() (tea.Model, tea.Cmd) {
	v := a.varList.SelectedVar()
	if v == nil || a.varList.IsHeaderAtCursor() {
		return a, a.flashMessage("No variable selected")
	}
	text := descriptionText(a.varList.File.DocComment(a.varList.SelectedVarIndex()), v.Comment)
	if text == "" {
		return a, a.flashMessage("No description for " + v.Key)
	}
	a.descKey = v.Key
	a.descText = text
	a.mode = ModeDescription
	return a, nil
}

// descriptionText joins wrapped comment lines into paragraphs. List items and
// indented lines keep their own line; the inline comment comes last.
func descriptionText(doc []string, inline string) string {
	var out []string // logical lines; "" separates paragraphs
	cur := ""
	flush := func() {
		if cur != "" {
			out = append(out, cur)
			cur = ""
		}
	}
	paragraphBreak := func() {
		flush()
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
	}
	for _, l := range doc {
		switch {
		case strings.TrimSpace(l) == "":
			paragraphBreak()
		case strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"):
			flush()
			out = append(out, l)
		case isListItem(l) || cur == "":
			flush()
			cur = l
		default:
			cur += " " + l
		}
	}
	if inline != "" {
		paragraphBreak()
		cur = inline
	}
	flush()
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// isListItem reports whether a comment line starts a bullet ("- ", "* ").
func isListItem(l string) bool {
	return strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")
}

func (a App) handleDescriptionKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.keys.Escape, a.keys.Describe, a.keys.Quit, a.keys.Enter) {
		a.mode = ModeNormal
		a.descKey = ""
		a.descText = ""
	}
	return a, nil
}

// overlay draws popup centered over base.
func (a App) overlay(base, popup string) string {
	x, y := a.popupOrigin(popup)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(popup).X(x).Y(y).Z(1),
	).Render()
}

// popupSize returns the rendered width and height of popup.
func popupSize(popup string) (w, h int) {
	return lipgloss.Width(popup), lipgloss.Height(popup)
}

// popupOrigin returns the top-left cell of popup when centered.
func (a App) popupOrigin(popup string) (x, y int) {
	w, h := popupSize(popup)
	return max(0, (a.width-w)/2), max(0, (a.height-h)/2)
}

func (a App) viewDescription() string {
	outer := min(descriptionMaxWidth, a.width)
	inner := max(1, outer-popupChromeWidth)

	title := a.theme.KeyStyle.Render(a.descKey)
	body := a.theme.NormalItem.Width(inner).Render(a.descText)
	hint := a.theme.MutedItem.Render("esc/i to close")

	// Clip the body to fit the terminal.
	maxBody := a.height - popupChromeHeight - descriptionFixedRows
	if bodyLines := strings.Split(body, "\n"); maxBody > 0 && len(bodyLines) > maxBody {
		body = strings.Join(bodyLines[:maxBody-1], "\n") + "\n" + a.theme.MutedItem.Render("…")
	}

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(a.theme.ColorPrimary).
		Padding(1, 2)

	return box.Render(lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", hint))
}
