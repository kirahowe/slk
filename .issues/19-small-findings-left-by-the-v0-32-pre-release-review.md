# Small findings left by the v0.32 pre-release review

## Context

Noticed by the review and fix agents on 2026-11-11 and left alone to
keep each fix to its subject:

- A typed URL after a tab ("x\thttps://…") is not linked. goldmark's
  Linkify starts a URL at a line head or after a space, `*`, `_`, `~`
  or `(`, and `afterPunctURL` adds only quote marks and colons.
- `PresenceChangeMsg` carries no workspace, so a change from a
  workspace that is not on screen reaches the sidebar and finder rows
  that are. A Slack user ID is the same person everywhere, so the dot
  is right; `UserStatusChangeMsg` does carry a `TeamID` and checks it.
- The sidebar puts remembered live presence back on every `SetItems`
  (`presenceByUser`). The channel finder's `SetItems` does not, so a
  presence change that arrives before the finder has its rows is lost
  to the finder until the next change or workspace switch.
- `sidebar.UpdatePresenceByUser` stops at the first row for a user,
  while the status path updates every row.
- `emoji.Match` lists one alias of a used emoji. If a history reload
  makes the highlighted alias share a row with another spelling
  (`thumbsup` and `+1`), the picker's highlight is clamped to its row
  number and does not follow the emoji.
- A space typed into the reaction picker's filter matches nothing,
  since no emoji name has one.

## Acceptance criteria

- [ ] Each bullet is fixed or struck out with a reason.

## Plan

## Log
