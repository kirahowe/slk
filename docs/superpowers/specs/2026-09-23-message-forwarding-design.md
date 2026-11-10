# Message forwarding

## UX

In normal mode, `F` forwards the selected channel message or thread reply.
Reuse the Ctrl+T channel finder, titled **Forward message to…**, with its
existing fuzzy matching, recency order, arrow/Ctrl+N/Ctrl+P navigation and
mouse selection. Offer joined channels and existing DMs/group DMs in the
current workspace; omit synthetic view shortcuts and non-joined channels.
Forwarding must not implicitly join a channel. Enter on a forwarded permalink message navigates to the original message, including selecting the exact reply when the source is a thread.

Enter (or a result click) sends once and closes the picker. Esc or an outside
click cancels without sending. Leave the current channel, selection, focus,
thread and compose draft unchanged. Report success or failure in a toast.
No selection means no action; the Threads list itself is not a message pane
(its focused thread panel is supported). Mode changes abandon an unsubmitted forward.

## Transport and boundaries

Capture the source workspace, channel and exact selected timestamp on opening,
including a reply's timestamp rather than its parent's. Invoke a new
`core.MessageService.Forward` operation from a Bubble Tea command, with a
bounded context. Production wiring resolves the captured workspace by ID,
never whichever workspace happens to be active when the command runs.

Share the message through `chat.shareMessage`, the call the web client's
Forward makes (`timestamp` = source ts, `channel` = source, `share_channel` =
destination, a fresh `client_msg_id` UUID, empty `text`; capture in
`internal/slack/testdata/chat_share_message.json`). Slack posts a real shared
message. Do not flatten or re-author the source body, run rich-text
conversion, or reuse compose's send-success/failure reducers. Slack controls
source-access permissions; this is not a copy that grants access to a private
source. The share is never retried, a 429 included, since a retry after Slack
accepted it would post it twice. The service returns the posted timestamp
and, best effort, the source permalink (fetched after the share; a failure
there is not a forward failure).

## Rows in the destination

A forward is one more pending insert in `pendingOps`, in the destination's
top level, started in the Update of the key that picked the destination. Its
placeholder ("Forwarded message", under a `local:N` TS) shows at once in
every window on the destination, and while it is pending the user's own
messages there are held, as for a send. The result (`reduceForwarded`) ends
it with `pendingOps.EndForward`:

- On success the placeholder settles with the share's own echo if that
  arrived first and was held (only Slack's copy carries the shared
  attachment). Otherwise it settles with a stand-in that shows the permalink
  as a link, or "Forwarded message" without one, and the TS is recorded as
  awaiting its echo. When the echo comes, `ForwardEcho` picks it out by that
  exact TS, ahead of the self-send checks, and it replaces the stand-in.
- On failure the placeholder is removed and a toast says why.
- Either way the held messages are released, so a share Slack posted despite
  a failed call shows from its echo.

The forwarded message therefore shows once in every window on the
destination, in either arrival order, and another send pending there is
matched only by its own TS. Compose state is never touched.

## Tests

- Finder forwarding scope, matching/order, navigation/clicks, updates, and
  restoration of the ordinary switcher.
- Channel and thread sources; captured identity; destination dispatch;
  no navigation/draft mutation; success/error feedback; cancellation and
  mode/workspace transitions; normal Ctrl+T behavior after forwarding.
- Core adapter forwarding and unsupported-operation error.
- Slack request fields against the capture, rejected shares, empty timestamp,
  no retry on a 429, and a permalink failure after a successful share.
- The destination shows the forward once, as Slack's copy, in every order of
  its result and echo, with and without an unrelated send pending there
  (through `Update`); a failed forward leaves no row; the pendingOps model
  property covers forwards beside sends.
- Composition-root wiring resolves the source workspace by ID.
