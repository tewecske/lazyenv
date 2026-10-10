package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lazynop/lazyenv/internal/config"
	"github.com/lazynop/lazyenv/internal/parser"
)

func newDescriptionTestApp(t *testing.T, content string) App {
	t.Helper()
	f := parser.ParseBytes(".env", []byte(content), config.SecretsConfig{})
	require.NotEmpty(t, f.Vars)
	app := newTestApp(nil)
	app.fileList.Files = append(app.fileList.Files, f)
	app.varList.SetFile(f)
	app.focus = FocusVars
	return app
}

func pressKey(app App, msg tea.KeyPressMsg) App {
	updated, _ := app.Update(msg)
	return updated.(App)
}

func TestDescriptionOpensAndCloses(t *testing.T) {
	app := newDescriptionTestApp(t, "# Database host.\nDB_HOST=localhost # overridden in CI\n")

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})
	assert.Equal(t, ModeDescription, app.mode)
	assert.Equal(t, "DB_HOST", app.descKey)
	assert.Equal(t, "Database host.\n\noverridden in CI", app.descText)

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})
	assert.Equal(t, ModeNormal, app.mode, "i should close the popup")

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})
	app = pressKey(app, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, ModeNormal, app.mode, "esc should close the popup")
	assert.Empty(t, app.descText)
}

func TestDescriptionIgnoresOtherKeys(t *testing.T) {
	app := newDescriptionTestApp(t, "# doc\nA=1\nB=2\n")

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})
	app = pressKey(app, tea.KeyPressMsg{Text: "j"})
	app = pressKey(app, tea.KeyPressMsg{Text: "d"})

	assert.Equal(t, ModeDescription, app.mode)
	assert.Equal(t, "A", app.varList.SelectedVar().Key, "cursor must not move under the popup")
	assert.Len(t, app.varList.File.Vars, 2)
}

func TestDescriptionNoComment(t *testing.T) {
	app := newDescriptionTestApp(t, "A=1\n")

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	assert.Equal(t, ModeNormal, app.mode)
	assert.Contains(t, app.statusBar.Message, "No description for A")
}

func TestDescriptionNoOpOnFilePanel(t *testing.T) {
	app := newDescriptionTestApp(t, "# doc\nA=1\n")
	app.focus = FocusFiles

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	assert.Equal(t, ModeNormal, app.mode)
}

func TestDescriptionWorksInReadOnly(t *testing.T) {
	app := newDescriptionTestApp(t, "# doc\nA=1\n")
	app.config.ReadOnly = true

	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	assert.Equal(t, ModeDescription, app.mode)
}

func TestDescriptionClickCloses(t *testing.T) {
	app := newDescriptionTestApp(t, "# doc\nA=1\n")
	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	updated, _ := app.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 1, Y: 1})
	app = updated.(App)

	assert.Equal(t, ModeNormal, app.mode)
	assert.Equal(t, FocusVars, app.focus, "closing click must not change focus")
}

func TestDescriptionViewRendersPopup(t *testing.T) {
	app := newDescriptionTestApp(t, "# Database host used by the API.\nDB_HOST=localhost\n")
	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	content := app.View().Content

	assert.Contains(t, content, "DB_HOST")
	assert.Contains(t, content, "Database host used by the API.")
	assert.Contains(t, content, "esc/i to close")
}

func TestDescriptionViewClipsToHeight(t *testing.T) {
	var content strings.Builder
	for range 50 {
		content.WriteString("# paragraph\n#\n")
	}
	app := newDescriptionTestApp(t, content.String()+"A=1\n")
	app.height = 20
	app = pressKey(app, tea.KeyPressMsg{Text: "i"})

	popup := app.viewDescription()

	assert.LessOrEqual(t, lipgloss.Height(popup), app.height)
	assert.Contains(t, popup, "…")
}

func TestDescriptionTextReflow(t *testing.T) {
	tests := []struct {
		name   string
		doc    []string
		inline string
		want   string
	}{
		{"empty", nil, "", ""},
		{"inline only", nil, "port", "port"},
		{"joins wrapped prose", []string{"Database host used", "by the API."}, "", "Database host used by the API."},
		{"blank line splits paragraphs", []string{"First.", "", "Second."}, "", "First.\n\nSecond."},
		{"collapses repeated blanks", []string{"", "First.", "", "", "Second.", ""}, "", "First.\n\nSecond."},
		{"list items keep their lines", []string{"Modes:", "- dev", "- prod"}, "", "Modes:\n- dev\n- prod"},
		{"indented lines are preformatted", []string{"Example:", "  a=1", "  b=2"}, "", "Example:\n  a=1\n  b=2"},
		{"inline after block", []string{"Host."}, "set in CI", "Host.\n\nset in CI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, descriptionText(tt.doc, tt.inline))
		})
	}
}
