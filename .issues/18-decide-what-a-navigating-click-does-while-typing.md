# Decide what a navigating click does while typing

## Context

Since 2026-11-11 a click while a composer has the keyboard changes
neither the focused pane nor the conversation on screen
(`(*App).clickFocus` and `navigates` in `internal/ui/reducer_mouse.go`).
A click on a sidebar row, a workspace tile, a Threads card or an
Activity item does nothing in insert mode, and a click on a message row
selects it without opening its thread.

That replaced a bug: those clicks used to navigate and leave insert
mode on with the keys going to a composer that was not focused, so the
next characters were dropped and Enter could send another box's draft.
Doing nothing is the safe rule, and it matches the keyboard, which
cannot navigate without leaving insert mode first.

Someone using the mouse may expect the click to work. It could leave
insert mode first, as Esc does, and then navigate: a draft stays with
its conversation, so nothing is lost. An edit in progress is the
exception, since leaving insert mode cancels it, and an upload in
flight already refuses navigation.

## Acceptance criteria

- [ ] Either a navigating click while typing a draft leaves insert mode and navigates, with the edit and upload cases refused and tested, or this is closed with the reason the click stays inert.

## Plan

## Log
