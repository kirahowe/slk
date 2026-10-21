# Mark conversations that have a canvas

## Context

Nothing in slk shows that a conversation has a canvas tab (a channel
canvas, or a DM's 1:1 notes). The user finds out only by pressing `c`
and getting either the canvas or "No canvas in this conversation".

Knowing up front needs each conversation's `properties` (`canvas`,
`tabs[]` with `type: "canvas"`, `meeting_notes`). userBoot already sends
them for every conversation, and `internal/slack/boot/boot.go` drops
`properties` on purpose (see its `Channel` struct comment), as does the
edgeapi `channels/info` decoder. `ConversationCanvases`
(`internal/slack/canvas.go`) shows the shape to decode.

## Acceptance criteria

- [ ] The pane header (or the sidebar row, to be decided) shows a mark
      for a conversation with at least one canvas.
- [ ] The mark appears at startup without an extra request per
      conversation.
- [ ] A canvas tab added or removed while slk runs updates the mark, or
      the issue records why it cannot (no WebSocket event found).

## Plan

- Keep the canvas file IDs from userBoot's `properties` on the
  conversation metadata, through the existing snapshot paths
  (`publishChannels`, no shared mutable map).
- Decide where the mark goes; the pane header is the least crowded.
- Check whether Slack sends an event when a tab is added (the web
  client's socket traffic would tell).
- The viewer could then skip `conversations.info` when the IDs are
  known.

## Log
