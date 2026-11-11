# Decide what Enter does on a shared message

## Context

`openForwardedSelected` (`internal/ui/forward_navigation.go`) makes
Enter go to the original message when the selected message's text is
exactly one Slack permalink. That was what a forward made by slk looked
like until forwarding moved to `chat.shareMessage` (upstream #424,
ported 2026-11-10).

A forward now arrives as Slack's own shared message: no text, and the
original in an attachment whose `from_url` is its permalink (the shape
`messages.MessageLinks` reads). Enter on it opens the forward's thread,
as on any message, and `o` offers the original. The stand-in row slk
shows until Slack's copy arrives is still a permalink (`forwardBody`),
so for that moment Enter goes to the original.

The forwarding spec
(`docs/superpowers/specs/2026-09-23-message-forwarding-design.md`)
still says "Enter on a forwarded permalink message navigates to the
original message". `wiki/Features.md` was corrected on 2026-11-11 to
say what the code does.

The choice has a cost either way. If Enter goes to the original on
every share without a comment, that covers messages other people shared
from Slack's client too, and those often have a discussion in their
thread that Enter would no longer open. If Enter keeps opening the
thread, the original is one key further away (`o`).

## Acceptance criteria

- [ ] Enter on a shared message does one thing whether the row is slk's stand-in or Slack's copy, and the spec and wiki say which.
- [ ] If Enter goes to the original, a share that has replies still has a way to open its thread.

## Plan

## Log
