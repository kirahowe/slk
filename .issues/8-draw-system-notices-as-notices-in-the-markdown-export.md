# Draw system notices as notices in the markdown export

## Context

Join, leave and canvas-update messages (`core.IsSystemNoticeSubtype`)
are drawn as one muted line in both panes since the port of upstream
#319's join/leave subset (2026-11-10). The markdown export
(`internal/export`) still writes one as an ordinary message from its
author, with the raw text.

## Acceptance criteria

- [ ] The thread and channel exports write a notice as one line that reads as an event, not as a message someone wrote.
- [ ] Export index snippets and the mention scan skip notices, as `countMentions` does.

## Plan

## Log
