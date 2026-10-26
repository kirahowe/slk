package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui"
)

// TestApplyUISettings_VersionCommandShowsTheBuild: applyUISettings hands
// the App the build identity, so :version shows the version and short
// commit GoReleaser stamped. The commands :version returns (the toast's
// clear) are dropped, so the toast is still up when the frame is read.
func TestApplyUISettings_VersionCommandShowsTheBuild(t *testing.T) {
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() { version, commit = oldVersion, oldCommit })
	version = "v1.2.3"
	commit = "0123456789abcdef0123456789abcdef01234567"

	app := ui.NewApp()
	applyUISettings(app, uiSettings{})
	app.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	for _, r := range ":version" {
		app.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if frame := ansi.Strip(app.View().Content); !strings.Contains(frame, "slk v1.2.3 (commit 0123456)") {
		t.Fatalf("status bar should show the short build identity, got:\n%s", frame)
	}
}
