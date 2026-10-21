# Render checklists and tables in canvases

## Context

The canvas viewer renders the elements seen in Slack's canvas HTML
export: headings, paragraphs, bulleted and numbered lists, quotes, code,
bold, italic, links, user mentions and date chips. A scratch canvas
meant to include checklists, strikethrough, tables, dividers, emoji and
a #channel mention came back without any of them in the HTML
(2026-10-21). Either they were not added, or the export drops them.

`internal/canvas` keeps unknown elements as their text, so whatever
does arrive shows as plain text, never vanishes.

## Acceptance criteria

- [ ] A scratch canvas confirmed (in Slack's UI) to contain a checklist
      with checked and unchecked items, strikethrough, a table, a
      divider, an emoji and a #channel mention, fetched and inspected.
- [ ] Each element the HTML carries gets a fixture and a rendering:
      `☐`/`☑` checklist items, struck text, a table laid out in columns
      (or a readable fallback at narrow widths), a divider rule, emoji,
      `#channel` resolved through the App's channel names.
- [ ] Each element the export drops is recorded here, so nobody chases
      it again.

## Plan

Fixtures go in `internal/canvas/testdata/`, with IDs scrubbed (the
current fixture uses `U0123ABCDEF`). Extend the parser's property-test
generator with each new tag. Rendering goes in
`internal/ui/canvasview/render.go`; keep `TestRenderFillsWidth` passing
for tables at every width.

## Log
