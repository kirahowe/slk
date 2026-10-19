package ui

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/ui/styles"
)

// keyFromString builds the key message a terminal delivers for a binding's
// key string ("L", "alt+y", "ctrl+shift+y", "pgup", ...). ok is false for
// a string it cannot build.
func keyFromString(s string) (tea.KeyMsg, bool) {
	parts := strings.Split(s, "+")
	name := parts[len(parts)-1]
	if name == "" { // "ctrl++" style: the key is "+"
		name = "+"
	}
	var mod tea.KeyMod
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl":
			mod |= tea.ModCtrl
		case "alt":
			mod |= tea.ModAlt
		case "shift":
			mod |= tea.ModShift
		default:
			return nil, false
		}
	}
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab,
		"space": tea.KeySpace, "up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight, "pgup": tea.KeyPgUp,
		"pgdown": tea.KeyPgDown, "backspace": tea.KeyBackspace,
	}
	if c, ok := named[name]; ok {
		return tea.KeyPressMsg{Code: c, Mod: mod}, true
	}
	r, n := utf8.DecodeRuneInString(name)
	if n != len(name) {
		return nil, false
	}
	if mod == 0 {
		return keyPress(r), true
	}
	return tea.KeyPressMsg{Code: r, Mod: mod}, true
}

// keyMapReachExempt names the bound keys a normal-mode press in the
// Channels view rightly leaves alone, each with why.
var keyMapReachExempt = map[string]string{
	"SearchNext/n":           "steps through an active / search; there is none",
	"SearchPrev/N":           "steps through an active / search; there is none",
	"Escape/esc":             "clears a search, an edit or a thread; there is none",
	"NavBack/ctrl+h":         "walks visit history; there is none",
	"NavForward/ctrl+l":      "walks visit history; there is none",
	"NavForward/ctrl+k":      "walks visit history; there is none",
	"CloseThreadView/q":      "closes the thread pane; none is open",
	"ActivityUnread/u":       "only in the Activity view",
	"ToggleSection/space":    "toggles a sidebar section; the sidebar is not focused",
	"SaveThread/S":           "saves the thread pane's thread; it is not focused",
	"RespondButtons/b":       "answers an ephemeral's buttons; the message has none (toast)",
	"ToggleBroadcast/ctrl+o": "insert mode only",
	"OpenInEditor/ctrl+x":    "insert mode only",
	"ClearCompose/ctrl+u":    "insert mode only; in normal mode ctrl+u is HalfPageUp",
	"HalfPageUp/ctrl+u":      "scrolls up; the view is at the top",
	"PageUp/pgup":            "scrolls up; the view is at the top",
	"Down/j":                 "moves down; the last message is selected",
	"Down/down":              "moves down; the last message is selected",
	"PageDown/pgdown":        "scrolls down; the view is at the bottom",
	"HalfPageDown/ctrl+d":    "scrolls down; the view is at the bottom",
	"Bottom/G":               "jumps to the bottom; the view is there",
	"ToggleThread/ctrl+]":    "closes the thread pane; none is open",
}

// keyMapGlobalTheme names the bindings that apply a theme, whose cases
// need a theme list, start on Dark and restore it after.
var keyMapGlobalTheme = map[string]bool{"ThemeNext": true, "ThemePrev": true}

// TestEveryBoundKeyReachesAHandler presses every key of every KeyMap
// binding through App.Update in normal mode, in the Channels view with a
// message selected, and requires each to do something: return a command,
// change the mode or change the screen. A key an earlier arm swallows, a
// binding whose arm went missing, or a key string the terminal never
// sends in that form fails here. Bindings with no keys are help-only.
// The cases run one at a time: each renders, and the renderer caches
// colours in package state that only one goroutine writes in the app.
func TestEveryBoundKeyReachesAHandler(t *testing.T) {
	styles.Apply("dark", config.Theme{})
	t.Cleanup(func() { styles.Apply("dark", config.Theme{}) })
	km := DefaultKeyMap()
	v := reflect.ValueOf(km)
	for i := range v.NumField() {
		field := v.Type().Field(i).Name
		b, ok := v.Field(i).Interface().(key.Binding)
		if !ok {
			continue
		}
		for _, ks := range b.Keys() {
			name := field + "/" + ks
			if _, exempt := keyMapReachExempt[name]; exempt {
				continue
			}
			t.Run(name, func(t *testing.T) {
				msg, ok := keyFromString(ks)
				if !ok {
					t.Fatalf("cannot build a key message for %q", ks)
				}
				if got := msg.String(); got != ks {
					t.Fatalf("built key reads %q, want %q", got, ks)
				}
				a, _ := listViewKeyApp(t, ViewChannels)
				if keyMapGlobalTheme[field] {
					startThemeCycle(t, a, "Dark", themeSwitcherItems())
				}
				before := a.View().Content
				_, cmd := a.Update(msg)
				if cmd == nil && a.mode == ModeNormal && a.View().Content == before {
					t.Errorf("%s did nothing", ks)
				}
			})
		}
	}
}

// Every exemption names a key that is still bound, so the list cannot
// outlive the bindings it excuses.
func TestKeyMapReachExemptionsAreBound(t *testing.T) {
	km := DefaultKeyMap()
	v := reflect.ValueOf(km)
	bound := map[string]bool{}
	for i := range v.NumField() {
		if b, ok := v.Field(i).Interface().(key.Binding); ok {
			for _, ks := range b.Keys() {
				bound[v.Type().Field(i).Name+"/"+ks] = true
			}
		}
	}
	for name := range keyMapReachExempt {
		if !bound[name] {
			t.Errorf("exemption %q names no bound key", name)
		}
	}
}
