# Close the send-path gaps the edit round-trip properties found in mrkdwn.Convert

## Context

The round-trip properties written for the edit seed
(`internal/ui/messages/edit_source_prop_test.go`) had to skip or pin a
few inputs because `mrkdwn.Convert` itself sends something other than
what was typed:

- `walkList` writes every nested list after its whole parent list, so
  the mrkdwn text has the sub-items in the wrong place (the block is
  right). Pinned by `TestEditSourceNestedListOrder`.
- `emphasisDelimiter` falls back to italic when the emphasis starts
  with a node that is not text, so `` *`x` y* `` is sent as italic.
- A broadcast (`@here`) typed inside a code span loses the code style.
- `- ` followed by a bare code fence makes an empty list item, which
  is not valid rich text.

Separately, a literal `@alice` that is text, not a mention, becomes a
mention when an edit is saved, because `translateMentions` has no way
to mark a name as literal. Saving did this before the port too.

Two more turned up on 2026-11-11, when the round-trip properties were
tried on `proptest.ComposerTextWithMarkers` (lines continuing a list
item, a lone `>`, quote lines that are only a marker). In both the
mrkdwn fallback and the block disagree, and the properties failed in
16 of 20 runs, so they still draw from `ComposerText`:

- A quote's blank first or last line (a `>` line next to `> x`) is in
  the fallback and not in the block: `"- x\n>\n>\n> 0"`,
  `"> >\n>\n- 0"`, `"> -\n>\n- 0"`.
- An item's paragraph that follows its nested list is in the item's
  section in the block, and after the nested list in the fallback:
  `"- 0\n- 0\n    - 0\n  0"`. `nestedBeforeParentItem` in
  `edit_source_prop_test.go` does not recognise that shape.

## Acceptance criteria

- [ ] Each bullet has a failing test in `internal/slack/mrkdwn`, then a fix, and the matching skip or narrowed generator in the edit-seed properties is removed.
- [ ] The two 2026-11-11 gaps are closed and the round-trip properties draw from `ComposerTextWithMarkers`.
- [ ] A decision on literal `@name`: either a backslash escape that `translateMentions` honours and the edit seed writes, or a note here that it stays as is.

## Plan

## Log
