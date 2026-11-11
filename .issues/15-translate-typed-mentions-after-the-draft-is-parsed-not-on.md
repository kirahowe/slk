# Translate typed mentions after the draft is parsed, not on its raw text

## Context

`compose.translateMentions` rewrites `@name`, `#channel` and `@group`
in the draft's raw text, before `mrkdwn.Convert` parses it. It knows
where the wire tokens are and nothing else about the text, so it
cannot tell a mention from the same characters inside a URL or a code
span.

Found in the pre-release review on 2026-11-11: with a user named bob,
`[x](https://x.com/@bob)` is sent with `<@U1>` inside the link's URL.
A bare URL that ends in `/@name` is rewritten by the same rule, so a
Mastodon profile link (`https://mastodon.social/@kira`) breaks and
pings whoever has that display name.

Two bullets of slk#11 have the same cause: a literal `@alice` that is
text becomes a mention when an edit is saved, and `@here` typed in a
code span loses the code style.

Guarding each case on the raw text means a second copy of Convert's
grammar in the composer, which is how the wire-token guard went wrong
in the same review. The translation would be exact if it ran on the
text nodes Convert's walker produces, where links, code and typed URLs
are already known.

## Acceptance criteria

- [ ] A name inside a typed URL, a markdown link's URL or a code span is sent as typed, with a property over `proptest` composer text.
- [ ] A typed mention anywhere else is still sent as a mention, and the edit seed's round-trip properties still hold.
- [ ] The two slk#11 bullets are fixed here or left there with the reason.

## Plan

## Log
