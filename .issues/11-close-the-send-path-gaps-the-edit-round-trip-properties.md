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

## Acceptance criteria

- [ ] Each bullet has a failing test in `internal/slack/mrkdwn`, then a fix, and the matching skip or narrowed generator in the edit-seed properties is removed.
- [ ] A decision on literal `@name`: either a backslash escape that `translateMentions` honours and the edit seed writes, or a note here that it stays as is.

## Plan

## Log
