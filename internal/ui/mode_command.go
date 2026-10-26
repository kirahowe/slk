// internal/ui/mode_command.go
//
// Command-mode key handler: the vi-style ":" prompt
// (window-management design §5).
//
// Entry: enterCommandMode (bound to ':' in normal mode). Printable
// keys append to App.cmdline, mirrored into the status bar's
// command-line slot. Backspace edits — and cancels when the buffer
// is already empty, matching vim's backspace-past-':' behavior.
// Tab completes the command name from the registry
// (internal/ui/command.go), cycling through the matches as it is
// pressed again; Shift+Tab cycles backwards. Esc cancels. Enter
// executes the line through the same registry after returning to
// Normal, so commands that set their own mode (e.g. :ws) win.
package ui

import (
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// cmdCompletion is the state of an in-progress Tab completion at the
// ':' prompt: matches is the sorted list of command names the buffer
// matched when the cycle started, and idx is the candidate currently
// sitting in cmdline. Only a Tab press that found the buffer still on
// its candidate continues the cycle; every other key that touches the
// buffer, and every Tab that cannot cycle, resets this to its zero
// value, so a stale list can never be matched against a newer line.
type cmdCompletion struct {
	matches []string
	idx     int
}

// enterCommandMode switches to ModeCommand with an empty buffer
// and shows the bare ':' prompt in the status bar.
func (a *App) enterCommandMode() {
	a.setCmdline("")
	a.SetMode(ModeCommand)
}

// exitCommandMode clears the buffer and prompt and returns to
// Normal. Shared by the cancel and execute paths.
func (a *App) exitCommandMode() {
	a.clearCmdline()
	a.SetMode(ModeNormal)
}

// setCmdline replaces the command line and mirrors it into the status
// bar, so the prompt and the buffer cannot disagree. It also drops any
// Tab cycle, and with it the completion list the cycle was showing: the
// line they were matched against is gone.
func (a *App) setCmdline(line string) {
	a.cmdline = line
	a.cmdComplete = cmdCompletion{}
	a.statusbar.SetCommandLine(":" + line)
	a.statusbar.SetCommandCandidates(nil, 0)
}

// clearCmdline drops the command line and the prompt with it; leaving
// ModeCommand by any path goes through here, so the status bar cannot
// keep rendering a stale ':' line or a stale completion list.
func (a *App) clearCmdline() {
	a.cmdline = ""
	a.cmdComplete = cmdCompletion{}
	a.statusbar.SetCommandLine("")
	a.statusbar.SetCommandCandidates(nil, 0)
}

// prefixMatches returns the names that start with prefix, sorted, or nil
// when none do. names is not modified.
func prefixMatches(names []string, prefix string) []string {
	var matches []string
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	slices.Sort(matches)
	return matches
}

// nextCompletion is one Tab (backwards for Shift+Tab) at the prompt, as a
// pure function of the cycle in progress c, the buffer line and the
// command names. It returns the new cycle: the line to show is
// matches[idx]. A zero result means Tab completes nothing (a buffer
// holding a space names arguments, which are out of scope, or no name
// matches), and the caller keeps the line. A cycle continues only while
// the buffer still holds its candidate; otherwise a new one starts from
// the names matching the buffer, at the first match (the last going
// backwards).
func nextCompletion(c cmdCompletion, line string, backwards bool, names []string) cmdCompletion {
	if strings.Contains(line, " ") {
		return cmdCompletion{}
	}
	if c.idx < len(c.matches) && c.matches[c.idx] == line {
		next := c.idx + 1
		if backwards {
			next = c.idx - 1 + len(c.matches)
		}
		return cmdCompletion{matches: c.matches, idx: next % len(c.matches)}
	}
	matches := prefixMatches(names, line)
	if len(matches) == 0 {
		return cmdCompletion{}
	}
	idx := 0
	if backwards {
		idx = len(matches) - 1
	}
	return cmdCompletion{matches: matches, idx: idx}
}

// completeCommandName runs Tab (backwards for Shift+Tab) at the prompt
// through nextCompletion over the commands registry: it puts the next
// matching name into cmdline and remembers the cycle so the next Tab
// advances instead of restarting. A Tab that completes nothing only
// resets the cycle.
func completeCommandName(a *App, backwards bool) {
	c := nextCompletion(a.cmdComplete, a.cmdline, backwards, slices.Collect(maps.Keys(commands)))
	if c.matches == nil {
		a.cmdComplete = cmdCompletion{}
		a.statusbar.SetCommandCandidates(nil, 0)
		return
	}
	a.setCmdline(c.matches[c.idx])
	a.cmdComplete = c
	a.showCommandCandidates(c.matches, c.idx)
}

// showCommandCandidates mirrors a fresh match list into the status bar's
// wildmenu. A single match adds nothing to the prompt -- the name it
// completed is already on screen -- so only a real choice is shown.
func (a *App) showCommandCandidates(matches []string, idx int) {
	if len(matches) < 2 {
		a.statusbar.SetCommandCandidates(nil, 0)
		return
	}
	a.statusbar.SetCommandCandidates(matches, idx)
}

func handleCommandMode(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.Key().Code {
	case tea.KeyEscape:
		a.exitCommandMode()
		return nil
	case tea.KeyEnter:
		line := a.cmdline
		a.exitCommandMode()
		return executeCommand(a, line)
	case tea.KeyTab:
		// Completion ignores the shift state in key dispatch
		// everywhere else in this file, so Shift+Tab reaches this
		// arm too; the modifier only reverses the cycle.
		completeCommandName(a, msg.Key().Mod.Contains(tea.ModShift))
		return nil
	case tea.KeyBackspace:
		if a.cmdline == "" {
			a.exitCommandMode()
			return nil
		}
		a.setCmdline(a.cmdline[:len(a.cmdline)-1])
		return nil
	}
	s := msg.String()
	if s == "space" {
		s = " "
	}
	if len(s) == 1 && s[0] >= 32 && s[0] <= 126 {
		a.setCmdline(a.cmdline + s)
	}
	return nil
}
