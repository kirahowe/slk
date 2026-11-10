# Draft preview above the compose box

Ported from upstream gammons/slk #453 (SLK-908), whose design is
`docs/superpowers/specs/2026-11-09-compose-draft-preview-design.md` on
upstream's main. This note records what slk does and where it differs.

## What it does

In insert mode, `ctrl+r` shows the draft in a few rows directly above
the compose box, inside the same pane border, drawn the way slk will
draw the message once it is sent: list items as bullets, bold as bold,
a link as its label, `@name` mentions as names. Pressing it again hides
it. "Preview" already names link previews and the image preview, so
this one is the draft preview everywhere.

```
 ...messages...
 alice is typing...
 ── draft preview ─────────────────────
 ▌Release plan:
 ▌• ship v0.28 on Tuesday
 ▌• read the notes

 ▌ Release plan:
 ▌ - ship **v0.28** on Tuesday
 ▌ - read [the notes](https://example.com/notes)
```

## Decisions taken from upstream

These were made by upstream's owner, and Kira takes them as given:

- Off by default. `ctrl+r` shows it and hides it.
- Sending the message closes it.
- The key is `ctrl+r`. Nothing in slk binds it, and the textarea's
  keymap (bubbles v2.1.0) does not use it.
- It is live: it changes as you type, with no debounce.
- Both composers have one, each with its own on/off state; the key
  only affects the box being typed in.
- With an empty draft it shows one dim line, "nothing to preview".
- It grows with the draft up to a cap of about a third of the pane's
  height, and past the cap shows the end of the draft.
- Leaving the compose box closes it.

Upstream's builder also made these calls, which slk keeps: a title rule
labelled "draft preview"; the body alone, without author line or
timestamp; a dim "…" first row when the top is cut off; a slash command
shows "slash command, not a message" (except while editing, since an
edit is saved as written); a caption sent with files shows "caption is
sent as typed", since an upload posts its caption unconverted; an open
picker sits between the preview and the box; `ctrl+u` clears the box
and keeps the preview; Esc that only closes a picker keeps it.

## What slk does differently, and why

- It draws the placeholder row, where upstream draws the converted
  text. A send in
  slk stores `messages.TypedBody` of the text: `mrkdwn.Convert`'s mrkdwn
  and its rich_text block. The pane draws a message from that block, so
  the placeholder Enter makes is drawn from it too. The preview builds
  that same row (`localRow` of `TypedBody` of the draft with mentions
  translated) and has the pane draw it with its own options
  (`RenderDraft` on `contentPane`, over `RenderOptions.BodyOnly`). The
  feature's main test is a property: for any draft, the preview's rows
  are the rows of the placeholder that Enter makes for it, drawn by the
  same pane at the same width, in both boxes. A second property, per
  pane, checks that a draft's rows are the pane's own row for the sent
  message below its header.
- The rows are the pane's rows. Each preview row has the row's
  invisible left border and fill and is exactly the pane's width, and
  beside an avatar the body keeps the avatar's gutter, blank. So a line
  breaks and sits where it will in the channel.
- Emoji images are drawn. Upstream turned image emoji off in the
  preview. slk draws them through the row's path in image mode and
  writes their kitty uploads as the frame draws, the way the thread
  parent's are written. A closed preview drops its rendered rows.
- Leaving insert mode closes it, by one rule. `SetMode` closes both
  boxes' previews whenever insert mode is left, which covers Esc, every
  send, saving or cancelling an edit, a slash command and a global
  intercept. A send and a move to another conversation (`SetDraftContext`)
  also close it through `clearVisibleDraft`.
- The cap is a third of the pane's inner height, at least two rows
  (the title and one line), and never more than leaves the message list
  its minimum of three rows. A short pane gets a short preview or none.
  With exactly two rows it shows the title and the draft's last line.
- The preview is part of the compose region. A click in the
  compose region enters insert mode from normal mode and does nothing in
  any other mode. The preview only exists in insert mode, so a click on
  it does nothing; before this, a click on the compose box in insert mode
  could select a message past the bottom of the window and open its
  thread.
- The composer caches the sorted mention list per change of
  its mention data, and the draft's translated text per change of the
  draft or that data. The rendered rows are cached on the translated
  text, the width, the pane's version, the window, the theme and the
  user's own avatar. With the preview closed, nothing extra runs.
