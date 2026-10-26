package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/help"
)

// versionToast runs :version the way a user does, typed at the prompt
// and sent with Enter, and returns the status bar's text afterwards.
func versionToast(t *testing.T, a *App) string {
	t.Helper()
	a.enterCommandMode()
	typeCommand(a, "version")
	if cmd := handleCommandMode(a, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil {
		t.Fatal(":version returned no command; its toast would never clear")
	}
	return stripANSI(a.statusbar.View(200))
}

// TestVersionCommandShowsBuildInfo: :version toasts the identity cmd/slk
// set with SetBuildInfo.
func TestVersionCommandShowsBuildInfo(t *testing.T) {
	a := NewApp()
	a.SetBuildInfo("slk v1.2.3 (commit abc1234)")
	if out := versionToast(t, a); !strings.Contains(out, "slk v1.2.3 (commit abc1234)") {
		t.Errorf("status bar should show the build identity, got:\n%s", out)
	}
}

// TestVersionCommandWithoutBuildInfo: an App cmd/slk never configured
// says so rather than toasting an empty line.
func TestVersionCommandWithoutBuildInfo(t *testing.T) {
	a := NewApp()
	if out := versionToast(t, a); !strings.Contains(out, "slk (unknown build)") {
		t.Errorf("status bar should show the unknown-build fallback, got:\n%s", out)
	}
}

// TestVersionCommandRepeatsTheUpdateHint: once the update check has
// reported a newer release, :version names it after the identity, so
// the hint can be found again after its own toast is gone.
func TestVersionCommandRepeatsTheUpdateHint(t *testing.T) {
	a := NewApp()
	a.SetBuildInfo("slk v1.2.3 (commit abc1234)")
	hint := "slk 1.3.0 is available. Run: brew upgrade --cask slk"
	a.Update(UpdateAvailableMsg{Version: "1.3.0", Hint: hint})
	a.statusbar.ClearCopied() // the check's own toast has gone

	out := versionToast(t, a)
	if !strings.Contains(out, "slk v1.2.3 (commit abc1234). "+hint) {
		t.Errorf("status bar should show the identity and the update hint, got:\n%s", out)
	}
}

// TestVersionCommandWithoutAnUpdate: an update check that found nothing
// newer (an empty hint) leaves :version showing the identity alone.
func TestVersionCommandWithoutAnUpdate(t *testing.T) {
	a := NewApp()
	a.SetBuildInfo("slk v1.2.3 (commit abc1234)")
	a.Update(UpdateAvailableMsg{})
	out := versionToast(t, a)
	if strings.Contains(out, "available") {
		t.Errorf("no update was found, but :version mentions one:\n%s", out)
	}
}

// TestHelpOverlayListsEveryCommand: the help overlay names every ":"
// command, so one added to the commands table without a help entry fails
// here instead of being findable only by Tab. Aliases are exempt.
func TestHelpOverlayListsEveryCommand(t *testing.T) {
	aliases := map[string]bool{"on": true, "qa": true} // :only, :quit
	var keys []string
	for _, e := range help.FromKeyMap(DefaultKeyMap()) {
		keys = append(keys, e.Key)
	}
	listed := " " + strings.Join(keys, " ") + " "
	for name := range commands {
		if aliases[name] {
			continue
		}
		if !strings.Contains(listed, " :"+name+" ") {
			t.Errorf(":%s has no entry in the help overlay", name)
		}
	}
}

// TestTabCompletesHelpAndVersion: the two commands are in the table Tab
// completes from.
func TestTabCompletesHelpAndVersion(t *testing.T) {
	for typed, want := range map[string]string{"he": "help", "ver": "version"} {
		a := NewApp()
		a.enterCommandMode()
		typeCommand(a, typed)
		pressTab(a, false)
		assertPrompt(t, a, want)
	}
}
