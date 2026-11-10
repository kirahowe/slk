# Check what Slack answers to a reply, edit, delete or forward on a join notice

## Context

`noticeGuard` (`internal/ui/notice_guard.go`) refuses Enter (open the
thread), a thread reply, `E`, `D` and `F` on a system notice with the
toast "Can't … a system message". The guard was added because those
actions make no sense on a notice, and because the user's own join
notice carries their user ID, so the ownership check alone let `E` and
`D` through to `chat.update` and `chat.delete`.

Nobody has checked what Slack does with each request, or which of
these Slack's own client offers on a join message. If it allows a
thread on one, the Enter guard is stricter than Slack.

## Acceptance criteria

- [ ] For each of the four actions, what Slack's client offers on a join message is written down here.
- [ ] A guard that is stricter than Slack is removed, or kept with the reason stated in its comment.

## Plan

## Log
