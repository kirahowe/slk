# Cache canvases so the viewer opens instantly

## Context

The canvas viewer (`internal/ui/canvas.go`, design in
`docs/superpowers/specs/2026-10-21-canvas-viewer-design.md`) fetches on
every open: `conversations.info` to list a conversation's canvases, then
`files.info` and the HTML download for the canvas. That is three
requests before anything shows, and nothing at all offline. The design
left caching out of the first version.

`files.info` returns `updated` (unix seconds), which says whether a
cached copy is still current.

## Acceptance criteria

- [ ] Opening a canvas seen before shows the cached copy at once, then
      replaces it if Slack has a newer one.
- [ ] The HTML is downloaded again only when `updated` has moved.
- [ ] A cached canvas whose refresh fails stays on screen, with the
      failure shown somewhere less disruptive than the error body.
- [ ] Cache rows are per workspace; nothing from one workspace shows in
      another.

## Plan

- A `canvases` table in `internal/cache` (`workspace_id`, `file_id`,
  `title`, `permalink`, `updated`, `html`), upserted like
  `internal/cache/marks.go`, created in `migrate()`.
- Store the HTML, not the parsed document, so a parser fix applies to
  cached canvases too.
- Follow `cachedThreadRepliesCmd`: a `FromCache` result renders first and
  the fetched one replaces it under the same viewer request number.
- Possibly cache each conversation's canvas file IDs too, which
  slk#3 also wants.

## Log
