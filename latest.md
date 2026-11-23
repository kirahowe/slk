# Upstream review 2026-11-23

Reviewed upstream `7d4f9b79..81dcdb05`, starting from the last report's `last_reviewed_upstream`. That commit is later than the ledger's reviewed-through `13f3bd0f`. The ledger has not been updated with the 2026-11-16 report's decisions yet, so those PRs (#419, #452, #454, #458, #459, #469 and others) are still open in that report and are not repeated here. The range has 16 merged PRs and 52 non-merge commits. None of them are already in the fork (`git cherry`) or in the ledger. The verdicts are 4 take, 10 maybe and 2 skip. The most important item is #479: the fork still posts a multi-file paste as one Slack message per file. `cmd/slk/main.go` calls `UploadFile` once per attachment, and each call does its own `files.completeUploadExternal`. The fix applies cleanly apart from its AGENTS.md row.

## Take

- **[#479](https://github.com/gammons/slk/pull/479): post multi-file pastes as one Slack message** (`511db0ca`)
  - What: adds `Client.UploadFiles` (`internal/slack/upload.go`). It gets an upload URL and PUTs the bytes for each file, then shares every file ID in one `files.completeUploadExternal` with the channel, the thread_ts and the caption. The compose send path calls it once per submit. `UploadFile` becomes the single-file case.
  - Why: this is a user-visible bug in the fork. Pasting three images posts three messages, and only the last one has the caption. Slack's web client posts one.
  - Conflict: low. Only `AGENTS.md` conflicts (the new table row); `cmd/slk/main.go` and `internal/slack/client.go` merge.
  - Cherry-pick: conflicts in AGENTS.md only. Its tests have no wall-clock waits. Check that the `SlackAPI` interface change (`UploadFileContext` replaced by the three V2 calls) doesn't break a fake elsewhere in the fork: run `go build ./... && go vet ./...` after picking.

- **[#483](https://github.com/gammons/slk/pull/483): report an unreadable cookie DB instead of "never signed in"** (`f990fc5d`)
  - What: `copyToTemp` returns `ErrCookieDBMissing` only for `fs.ErrNotExist`. Any other open error becomes the new `ErrCookieDBUnreadable`, which wraps the cause, and onboarding tells the user to grant Full Disk Access.
  - Why: on macOS, TCC lets stat succeed and then fails open with EPERM. The fork currently tells that user Slack has never signed in, which is wrong and sends them the wrong way.
  - Conflict: none.
  - Cherry-pick: clean. The permission test skips as root, so it won't run in CI containers that run as root.

- **[#490](https://github.com/gammons/slk/pull/490): auto-detect Rex as a kitty graphics terminal** (`0091101f`)
  - What: `TERM_PROGRAM=rex` is detected as kitty, like Ghostty. Rex is built on Ghostty's engine.
  - Why: a one-line detection fix with a test case. Without it, Rex users get half-block images.
  - Conflict: trivial. The fork's line is `env.TermProgram == "ghostty"`, without upstream's `"Nash"`, and the wiki table differs. Add `|| env.TermProgram == "rex"` by hand.
  - Cherry-pick: conflicts (`internal/image/capability.go`, `wiki/Terminal-Compatibility.md`), both one-line.

- **[#484](https://github.com/gammons/slk/pull/484): clock seam for membership TTL and failure backoff** (`a30b8eef`)
  - What: `membership.Manager` gets a `now func() time.Time` field (set to `time.Now` in `New`) for the TTL and `FailureBackoff` checks. The backoff-expiry test now advances that clock instead of backdating the unexported `lastFailed` map.
  - Why: it is test-only, but it is exactly what AGENTS.md asks for ("pass in the clock: `now` fields elsewhere"), and it is about 40 lines. Take it only if that alignment is worth a commit to you.
  - Conflict: low. The fork's `manager.go` has drifted in line numbers (`1f35d0df`). There are three `time.Since` sites, at lines 107, 177 and 186. The `time.Now().Unix()` writes at lines 231 and 259 stay as they are.
  - Cherry-pick: conflicts (`internal/slack/membership/manager.go`, `manager_test.go`). The change is mechanical.

## Maybe

- **[#487](https://github.com/gammons/slk/pull/487): highlight messages that mention me** (`47a2355d`, `b2263cb1`)
  - What: upstream catches up to the mention tint and bar the fork already draws (`RenderedMessage.Mention`, `styles.MentionBar`/`MentionBg`). It also adds two things the fork lacks. First, usergroup mentions of groups the user is in: `usergroups.list` with `include_users`, at no extra request. Second, mentions inside Block Kit blocks and legacy attachments, not only `Text`.
  - Judgement: do you want rows tinted for `@group` mentions and for bot messages whose mention sits in a block? `internal/mention/mention.go:30` deliberately skips usergroups today, because it does not know the user's memberships. Upstream solves that part.
  - Fork fit: port the two ideas onto `mention.Classify` and `RenderMessage`; don't take upstream's highlighting, colours or theme roles. Any new pure matcher needs a rapid property.
  - Conflict: high. It touches 12 files, including `render_message.go`, `wrap_rows.go`, `styles/themes.go` and `cmd/slk/workspace.go`.

- **[#502](https://github.com/gammons/slk/pull/502): stop re-wrapping message rows and dropping the whole cache** (`aba31989`, `06e210dd`, `eeef9ef0`)
  - What: `WrapRows` pads and borders rows by hand instead of calling lipgloss `Width().Render` on already-wrapped, OSC 8-heavy lines. The selected variant is built only for the cursor row. Name resolutions, status updates, emoji arrivals and read-mark echoes mark single entries stale (`messages/stale.go`) instead of niling the cache. Upstream measures `buildCache` at 268ms → 68ms on a link-heavy pane 378 columns wide.
  - Judgement: is a cache rebuild slow enough on your terminal to justify a 1500-line rework of the channel pane's hot path? The stale-entry half is the more valuable part: one name resolution currently rebuilds every row.
  - Fork fit: upstream's `WrapStyles` carries `compact` (#438 was declined). The fork's mention-row styles and `SelectedVariant` differ, so `cellDiff` against the current goldens is the acceptance test. Upstream adds a benchmark (`linkheavy_bench_test.go`); measure the fork with it before porting.
  - Conflict: high (`internal/ui/messages/model.go`, `wrap_rows.go`, `thread/model.go` and four tests).

- **[#495](https://github.com/gammons/slk/pull/495): sixel images in the thread pane, sixel avatars** (`4aef8a49`, `2fddd692`, `4ce161fd`, `5224e6eb`)
  - What: the channel pane's per-frame sixel placement becomes `messages.PlaceSixel`, which the thread pane now calls as well, so thread images are real sixel instead of the half-block splice. Avatars become sixel on sixel terminals through an optional `core.SixelAvatarService`. `4ce161fd` places thread images in the Threads and Activity views too.
  - Judgement: this only matters on sixel terminals (foot, mlterm, iTerm2 on sixel). If you use kitty or Ghostty, skip it. The sixel avatars also re-send a few KB per avatar on scroll.
  - Fork fit: the fork's thread pane relies on `RenderOptions.SixelRows = false` and `blockkit.SpliceSixelFallbacks`, which AGENTS.md documents. Taking this changes those rows. `4aef8a49` and `4ce161fd` can come without the avatars commit.
  - Conflict: high (`messages/model.go`, `render_message.go`, `thread/model.go`, AGENTS.md).

- **[#481](https://github.com/gammons/slk/pull/481): lay a message's images out side by side** (`41d18a8a`, `552cff7f`)
  - What: several image attachments share a row and wrap when the next one doesn't fit. Kitty, sixel and half-block all use the same cell grid. A single image is unchanged.
  - Judgement: a layout change for messages with several images, about 1170 lines. It saves vertical space; the cost is review surface on the image placement code the fork has already reworked (`bubbles/ansi.Graft`, the #362/#363 ports).
  - Conflict: high (`messages/model.go`, `render_message.go`, AGENTS.md). It overlaps #495; if you take both, take #495 first.

- **[#488](https://github.com/gammons/slk/pull/488): ctrl+g and `show_workspace_rail` hide the workspace rail** (`06588ac3`, `ffbfc72a`, `2fd73bcd`, `3267c0c7`)
  - What: a toggle and a config option to hide the rail. While it is hidden, the status bar shows the workspace's initials, and `h` from the sidebar stays put. `railWidth()` stays the only source of the width.
  - Judgement: the same question as #469 in the last report, but as a toggle. It saves columns for the narrow layout. Pick at most one of the two. `ctrl+g` is free in the fork.
  - Conflict: medium (`internal/ui/app.go`, `keys.go`, `mode_insert.go`, `config/config.go`, wiki). `3267c0c7` moves the status-bar label out of `View`.

- **[#496](https://github.com/gammons/slk/pull/496): channel finder lists unread conversations first before you type** (`5229d2f8`, `21721c11`)
  - What: with an empty query, joined conversations the sidebar shows as unread sort ahead of read ones, below a separator line. Forwarding mode is unaffected. `21721c11` adds `sidebar.UnreadChannelIDs`.
  - Judgement: is the fork's Unreads section and its status-bar pill enough, or do you also want ctrl+p to open on unread rows? It changes an ordering you may have muscle memory for.
  - Fork fit: the set should come from `ChannelItem.InUnreads` (the fork's one predicate), not from a new count. `SetUnread` keeps the map it is given, so hand it a copy. Its tests are example-only; a property such as "every unread joined row precedes every read joined row, and relative order is otherwise unchanged" would fit AGENTS.md.
  - Conflict: high (`channelfinder/model.go`, `sidebar/model.go`).

- **[#498](https://github.com/gammons/slk/pull/498): open a pasted Slack link from a dialog (P)** (`bffb03e1`, `37a58e71`, `0097971f`, `b34b7367`)
  - What: `P` in normal mode opens a one-line dialog. Pasting a permalink and pressing Enter routes it through `routeLink`, including the cross-workspace switch. `37a58e71` fixes loading a shared channel after a switch, and keeps a newer pending permalink when a late switch finishes.
  - Judgement: the fork already has `slk open <url>` over the unix socket, which covers opening from outside the terminal. This is the in-app paste path. Is that worth about 850 lines?
  - Fork fit: route through `routeLinkInApp`/`applyLocation` and the fork's `pendingLinkNav`. Check whether the shared-channel bug from `37a58e71` affects the fork's `applyLocation` on its own; if it does, that fix is worth taking separately.
  - Conflict: high (`app.go`, `channelfinder/model.go`, `reducer_links.go`, `reducer_workspace.go`).

- **[#485](https://github.com/gammons/slk/pull/485): `click_opens_thread` option** (`a92cfa3f`)
  - What: `[appearance] click_opens_thread = false` makes a click on a message row only select it. It is on by default.
  - Judgement: do you want clicks to stop opening threads? It's an option for one behaviour, like the declined #353. Lean skip unless a click opening a thread bothers you.
  - Conflict: medium (`app.go`, `config.go`, `ui_settings.go`, `demo.go`, wiki). The fork's `clickFocus` and insert-mode click rules (`reducer_mouse.go`) decide where the check goes.

- **[#475](https://github.com/gammons/slk/pull/475): workspace permalink domain aliases** (`9f2b4532`..`275b4112`)
  - What: a per-workspace list of extra Slack subdomains, so permalinks on an old or alias domain route in-app instead of opening the browser.
  - Judgement: only useful if one of your workspaces has been renamed or links come in on an alias subdomain. Lean skip otherwise.
  - Conflict: medium (`cmd/slk/main.go`, `internal/ui/msgs.go`); `a8292f2d` removes diagnostics the fork never had.

- **[#492](https://github.com/gammons/slk/pull/492): `:suppress` hides a bot's or user's messages per channel or workspace** (`afb9b52a`..`6cc649e9`)
  - What: rules stored in config.toml as `[[workspaces.<key>.suppress]]` blocks. A hidden author's messages are not drawn, never notify and never mark a channel unread. `:suppressed` lists the rules, and removing one reloads the panes from the cache. Eight follow-up fixes deal with reload, paging and delete edge cases.
  - Judgement: is muting a noisy bot worth about 2700 lines across 39 files? Most of it is the reload machinery. This is the same shape as #426, which was declined. Lean skip.
  - Fork fit if taken: it would have to go through `paneRows` (`TestPaneRowsGoThroughPaneRows`) and the fork's `pendingOps`. The config writes should reuse `saveWorkspaceSetting`'s find-by-team_id approach.

## Skip

- **[#489](https://github.com/gammons/slk/pull/489): boot Enterprise Grid member workspaces**: Grid only, about 2000 lines; no Grid workspace is in use (ledger #444, #416).
- **[#500](https://github.com/gammons/slk/pull/500): fold a duplicate team-id workspace block (first-run version_ts)**: already fixed in the fork. `saveWorkspaceSetting` finds the block by `team_id` in the file, never from a config loaded earlier. Upstream's load-time fold would only repair a config.toml broken by an older build. Conflicts in `cmd/slk/save_version_ts.go`.
