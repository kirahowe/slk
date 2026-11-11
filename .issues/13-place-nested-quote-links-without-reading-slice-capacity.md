# Place nested-quote links without reading slice capacity

## Context

`(*walker).autoLinksIn` (`internal/slack/mrkdwn/walk.go`) needs each
`ast.AutoLink`'s offset in the source to send a URL inside a nested
quote as a link. goldmark keeps the link's text segment unexported, so
the code computes the offset as `cap(source) - cap(label)`, relying on
`AutoLink.Label` returning a two-index subslice of the source.

Since 2026-11-11 the computation is in one place,
`(*walker).autoLinkStart`, and has a second caller: `typedAutoLink`,
which sends an autolink inside a link label as the text it was typed
as.

It checks the bytes at that offset and falls back to plain text when
they differ, and the nested-quote tests fail if goldmark ever stops
returning a subslice, so a goldmark upgrade cannot break it silently.
It is still an odd thing to rely on.

## Acceptance criteria

- [ ] The offset comes from something goldmark exports (a sibling text node's segment, or a small inline parser of ours that records the segment), or this is closed with the reason the current form stays.

## Plan

## Log
