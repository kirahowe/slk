# Canvas viewer

## Problem

Slack canvases are invisible in slk. A conversation's canvas tab (a
channel canvas, or the 1:1 notes canvas a manager keeps on a DM) does
not appear at all, and a canvas shared in a message is drawn as
`[File] <title>` whose only use is the terminal hyperlink to the
browser. Reading 1:1 notes means switching to Slack.

## Scope

Read-only. slk shows a canvas; editing it, appending to it and live
collaboration stay in Slack, reached with `o` from the viewer.

## What Slack provides

Checked against a live workspace on 2026-10-21.

- `conversations.info` lists a conversation's canvases under
  `properties`. A DM's notes canvas appears as a tab,
  `properties.tabs[] {id, type: "canvas", data: {file_id, shared_ts},
  label}`, and again as `properties.meeting_notes.file_id`. A channel
  canvas is expected at `properties.canvas.file_id` and as a canvas tab.
  slack-go's `Tab` has no `data`, so this response is decoded by hand.
- A canvas is a file. `files.info` returns `filetype: "quip"`,
  `mimetype: "application/vnd.slack-docs"`, `pretty_type: "Canvas"`,
  `title`, `permalink`, `updated` (unix seconds) and
  `url_private_download`.
- Downloading `url_private_download` with the session's token and
  cookie returns a small HTML document (`text/html; charset=utf-8`, no
  redirect, no script), rooted at `<div class="quip-canvas-content">`.

There is no official read API, and the web client edits over an
internal protocol. The download is the read path.

### The HTML

Seen so far, from a scratch canvas and from the structure (not the
text) of a real 1:1 canvas:

| Markup | Meaning |
|---|---|
| `h1`, `h2`, `h3` | headings |
| `p.line` | paragraph; empty for a blank line |
| `div[data-section-style=5] > ul > li > span` | bulleted list |
| `div[data-section-style=6] > ul > li > span` | numbered list; `li[value]` restarts numbering |
| `ul` nested directly in `ul` | one level of indent |
| `blockquote > p.line` | quote |
| `p.prettyprint` | code block line |
| `code` | inline code |
| `control[data-remapped]` | an inline chip; seen as a date ("Today" in Slack) |

Not yet seen: checklists, bold, italic, strikethrough, links, user and
channel mentions, tables, dividers, images, emoji. The parser keeps any
element it does not recognise as its text content, so an unseen
element degrades to plain text rather than vanishing. Each one gets a
proper rendering when a sample of it is captured.

## Design

### Parsing: `internal/canvas`

A new package, pure and free of I/O, parses the HTML (with
`golang.org/x/net/html`) into a small document model:

- `Document{Blocks []Block}`
- `Block` is one of heading (level), paragraph, list item (kind:
  bullet, numbered, checklist; depth; number; checked), quote, code
  line, or an unknown block holding its text.
- Each block holds `[]Span`; a span is text with marks (bold, italic,
  strike, code, link URL) or a chip (date, user, channel) carrying its
  text and any ID.

Consecutive code lines are grouped by the renderer, not the parser, so
the model stays one block per Slack line.

`internal/core` names the model in the canvas port, which is why it
lives outside `internal/ui`.

Tests:

- examples from fixtures in `internal/canvas/testdata/`: the scratch
  canvas as fetched, and one per element type as samples arrive. No
  fixture comes from a real conversation;
- a property test: every non-whitespace text node of the input appears,
  in order, in the document's plain text. This is what "nothing
  vanishes" means;
- a fuzz target: `Parse` never panics and the property above holds on
  arbitrary bytes.

### Rendering: `internal/ui/canvasview`

`Render(doc, width, ctx) []string` turns a document into styled lines:

- headings bold, `h1` in the accent colour, with a blank line before
  each heading after the first;
- bullets `•`, `◦`, `▪` by depth (cycling), numbered items `1.`,
  checklist items `☐` / `☑`, each with a hanging indent so wrapped lines
  line up under the text;
- a quote with a `│` bar, code lines grouped into a block with the code
  background, inline code with the inline code style;
- links as OSC 8 hyperlinks; a date chip as `📅 <text>`; user and
  channel chips resolved through the App's `userNames` and
  `channelNames`, as `@name` and `#name`;
- wrapping with `messages.WordWrap`; styles from `ui/styles`, read per
  draw so a theme change applies.

It is a pure function of the document, width and context, so it gets a
property test (no line wider than `width`; the plain text of the lines
is the document's text) and goldens through the dialog golden harness.

### Fetching: a `CanvasService` port

`internal/core` gains:

```go
type CanvasService interface {
    // ConversationCanvases lists a conversation's canvas file IDs:
    // properties.canvas first, then canvas tabs in tab order, without
    // duplicates.
    ConversationCanvases(ctx context.Context, teamID, channelID string) ([]string, error)
    Canvas(ctx context.Context, teamID, fileID string) (Canvas, error)
}

type Canvas struct {
    FileID, Title, Permalink string
    Updated                  time.Time
    Doc                      canvas.Document
}
```

with `CanvasServiceFuncs` / `NewCanvasService`, a no-op default in
`service_defaults.go` and `SetCanvasService`, following the profile
service.

`internal/slack` gains two `Client` methods. `ConversationCanvases`
calls `conversations.info` through `postForm` and decodes `properties`
itself. `CanvasHTML` calls `files.info` through `postForm`, then GETs
`url_private_download` with the client's cookie-authenticated HTTP
client and the bearer token. Fetching through the workspace's own
`Client` rather than `filedl` matters: `filedl` treats a 200 `text/html`
response as Slack's login page, which is exactly what a canvas is. The
login page is told apart by the missing `quip-canvas-content` root; a
response without it is an error, not an empty canvas.

`cmd/slk` wires both to the `Client` of the workspace named by `teamID`
(`router.ByID`), as the profile service does, and parses the HTML
before handing the `Canvas` to the UI.

### The viewer

A full-screen overlay, `ModeCanvas`, with its model in
`internal/ui/canvasview`. It follows the profile dialog for the
asynchronous fetch (a seed, loading / loaded / failed states, results
for a different target ignored) and the reactions view for scrolling.

- The box fills the terminal less a margin of two cells on each side,
  over the dimmed screen. Text is wrapped to at most 100 columns and
  centred in the box when the terminal is wider, for readability.
- The title is the canvas title. The footer shows the keys and, when
  the conversation has more than one canvas, "1 of 2".
- Keys: `j`/`k` and arrows scroll a line, `ctrl+d`/`ctrl+u` half a page,
  `pgdown`/`pgup` and space a page, `g`/`G` top and bottom, `tab` /
  `shift+tab` the next and previous canvas, `o` opens the permalink in
  the browser or Slack, `r` re-fetches, and `esc`, `q` or `c` close.
  The mouse wheel scrolls (it does for every mode in
  `IsModalOverlay`).
- A scrollbar gutter from `bubbles/scrollbar` when the text is taller
  than the box.
- Switching workspace closes it, as it closes the profile dialog.

Loading shows "Loading canvas…" in the box. A failure shows the error
in the box, with `r` to retry and `o` still available once a permalink
is known.

### Opening it

`c` in normal mode (field `Canvas` in `KeyMap`, help group Views):

1. If the selected message has a canvas attachment, open that canvas.
2. Otherwise open the focused conversation's canvases. If it has none,
   toast "No canvas in this conversation".

The selection rule makes `c` on a shared canvas do the obvious thing
without a second key. `:canvas` does step 2 alone, and a help-only
binding lists it.

With no canvas service wired, `c` toasts, so
`TestEveryBoundKeyReachesAHandler` passes without an exemption.

### Canvas attachments

`extractAttachments` marks a file whose `filetype` is `quip` or whose
mimetype is `application/vnd.slack-docs` with kind `"canvas"` and sets
its `FileID`. Every message path (live, fetched, and cached via
`raw_json`) goes through that function, so cached messages pick this
up with no migration.

`renderSingleAttachment` draws `[Canvas] <title>`. `d` already collects
only `Kind == "file"`, so it stops offering to download a canvas as an
HTML file.

### Demo mode

The demo workspace gets a DM with a notes canvas and a channel message
sharing one, from a fixture, so the viewer can be recorded and checked
in tmux without a real workspace.

## Not in this version

- **Caching.** Each open fetches `files.info` and the HTML (two small
  requests). A `canvases` table keyed by workspace and file ID, holding
  the HTML and `updated`, would show the cached copy at once and skip
  the download when `updated` has not moved. Worth adding if opening
  feels slow.
- **An indicator that a conversation has a canvas.** It needs
  `properties` for every conversation up front. userBoot already sends
  them, and `internal/slack/boot` drops them on purpose; keeping
  `properties.tabs` there would let the pane header show a canvas mark.
- **Relative dates on date chips.** Slack shows "Today"; the HTML holds
  a fixed date string. If the chip's attributes carry a timestamp,
  rendering it relative to the clock is a small follow-up.
- Editing, appending and comments.

## Phases

Each phase lands green on its own.

1. `internal/canvas`: model, parser, fixtures, property and fuzz tests.
2. `internal/ui/canvasview` rendering, with its property test.
3. `CanvasService`: `internal/slack` methods, the core port, `cmd/slk`
   wiring, tests against recorded responses through `handlerClient`.
4. The viewer: mode, model, keys, `:canvas`, overlay registration,
   goldens.
5. Canvas attachments: kind, `[Canvas]` label, `c` on a selected
   message.
6. Demo fixture; QA in tmux under `slk --demo`.
