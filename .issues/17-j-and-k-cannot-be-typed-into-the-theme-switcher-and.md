# j and k cannot be typed into the theme switcher and presence menu filters

## Context

Both dialogs show a filter box ("Type to filter...") and both bind `j`
and `k` to move the selection (`internal/ui/themeswitcher/model.go`,
`internal/ui/presencemenu/model.go`: `case "down", "ctrl+n", "j"` and
`case "up", "ctrl+p", "k"`), ahead of the branch that adds a typed
character to the query. So neither letter reaches the filter: typing
"dark" in the theme switcher filters on "dar" and moves the selection
up one row.

Found in the pre-release review on 2026-11-11 and left alone, because
the fix takes a key away. The channel and workspace finders move with
the arrows and Ctrl+N / Ctrl+P only, and every letter types. Doing the
same here ends `j` / `k` movement in these two dialogs, which someone
who steps through themes with `j` to preview them would notice.

## Acceptance criteria

- [ ] Every letter can be typed into both filters, with a test per dialog that types a name containing `j` and `k`.
- [ ] The movement keys each dialog keeps are the ones its help text and `wiki/Keybindings.md` name.

## Plan

## Log
