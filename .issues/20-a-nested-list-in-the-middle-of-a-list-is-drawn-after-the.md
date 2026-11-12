# A nested list in the middle of a list is drawn after the whole list

## Context

`mrkdwn.Convert` puts every nested list after its whole parent list in
the rich_text block (`walkList`, `internal/slack/mrkdwn/walk.go`).
Typed as

    - a
        - b
    - c

the block is the list `a`, `c` and then the list `b` at indent 1, so
Slack draws a, c, b. The mrkdwn fallback is in the typed order, but
Slack's clients draw the block. Checked on 2026-11-12 at v0.32.0;
slk#11's first bullet described it the other way round and now points
here.

A fix was written and never landed: commit `d9b11e0d`, "Send a nested
list where it was typed", on the 2026-11-02 upstream branch. PR #2
carried it and was closed on 2026-11-12, because everything else in it
had reached main by another route. The commit puts a nested list
after the item that holds it and continues the parent's later items in
a new list, an ordered one counting on through its offset, which it
says is the shape Slack's own client sends. `git fetch origin
pull/2/head` brings it back.

It will not apply as it is. `walkList` has changed since (an item's
paragraph after a nested list, the breaks after an item's quote), and
the edit seed is built for the current block shape:
`TestEditSourceNestedListOrder` pins the seed listing the nested list
last, and `nestedBeforeParentItem` excuses the fallback's order in the
round-trip property.

## Acceptance criteria

- [ ] A nested list between two items of its parent is drawn between them, for bullet and ordered parents and two levels down, with example tests in `internal/slack/mrkdwn`.
- [ ] The edit seed of such a message sends the same block, `TestEditSourceNestedListOrder` states the typed order, and `nestedBeforeParentItem` is gone from the round-trip property.

## Plan

## Log
