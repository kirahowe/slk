# Keep Graft's trailing escapes and row style at the edges

## Context

`bubbles/ansi.Graft` (`internal/bubbles/ansi/ansi.go`) says the line
keeps its own escapes on both sides of the insert. Two cases break that:

- When the insert reaches or passes the line's last cell (`end >= w`),
  it returns `prefix + insert`. Any escapes after the last cell, such as
  the closing SGR reset or an OSC 8 terminator, are dropped, so an open
  hyperlink or background can carry on past the row.
- When a wide character straddles the insert's right edge, the cell left
  over becomes a bare `" "` with no style. On a tinted row (the
  selection) or the theme background, that one cell shows the terminal's
  default background.

Found in the pre-release review of v0.30.0..HEAD on 2026-11-09. Neither
case is known to show today: the callers (`SpliceSixelFallbacks`, the
channel pane's partly visible sixel images) graft halfblock rows over
blank placeholder rows that stay inside the line, so the insert does not
reach the end and no wide character sits at its edge.

## Acceptance criteria

- [ ] With `end >= w`, the line's escapes after its last cell are kept
      after the insert.
- [ ] The cell left by a straddling wide character carries the style in
      effect at that column in the line.
- [ ] Graft's property test in `internal/bubbles/ansi` covers both cases.
