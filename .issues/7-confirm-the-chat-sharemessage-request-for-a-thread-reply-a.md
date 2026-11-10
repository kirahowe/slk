# Confirm the chat.shareMessage request for a thread reply, a DM and a re-forward

## Context

Forwarding now calls `chat.shareMessage` (`internal/slack/forward.go`,
ported from upstream #424 on 2026-11-10). The request is checked
against one scrubbed web-client capture,
`internal/slack/testdata/chat_share_message.json`, which shows a
top-level message shared from one channel into another.

Three cases send the same fields as that capture because upstream's
code does, and nothing confirms the web client sends nothing more:

- forwarding a thread reply (the reply's ts goes in `timestamp`, no
  `thread_ts` is sent);
- forwarding into a DM (a `D…` id in `share_channel`);
- forwarding a message that is itself a forward.

The UI also assumes the share's echo is an ordinary top-level message
from the user with the TS the HTTP result returned.

A failed forward shows a toast and leaves no row, so a wrong request
is visible, not silent.

## Acceptance criteria

- [ ] Each of the three cases is forwarded once from slk and arrives as a shared message in Slack's own client.
- [ ] A web-client capture of at least the thread-reply case is scrubbed into `testdata/` and the request test checks against it, or the difference is fixed.

## Plan

## Log
