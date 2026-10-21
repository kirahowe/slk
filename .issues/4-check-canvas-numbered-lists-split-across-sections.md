# Check canvas numbered lists split across sections

## Context

`internal/canvas` restarts list numbering at every
`<div data-section-style=…>` it meets, and honours `li[value]` as an
explicit start. Every canvas seen so far wraps each list in one such div
and puts `value='1'` on the first item, so this has been right.

Unverified: whether Slack ever splits one numbered list across several
section divs (for example a numbered list interrupted by a nested
bulleted one, or a long list edited in pieces) without a `value` on the
continuing part. If it does, slk would number the second part from 1
where Slack continues.

## Acceptance criteria

- [ ] A real canvas with a numbered list interrupted by other content,
      and one with a numbered list that has a nested bulleted list,
      fetched and its HTML inspected.
- [ ] Either a scrubbed fixture in `internal/canvas/testdata/` with a
      test pinning the right numbering (fixing the parser if needed), or
      a note here that Slack always restarts or always marks `value`.

## Plan

Build the canvas in a scratch DM, fetch `url_private_download` as the
design doc describes, and compare with what Slack draws. Do not use a
real conversation's canvas as a fixture.

## Log
