# Upstream review 2026-10-26

Range: `07bdfab0..04d0a2d4` (ledger and last report both stopped at `07bdfab0`). 23 upstream PRs, 75 non-merge commits; none of them are already in the fork by patch, and none are in the ledger. Counts: 11 take, 7 maybe, 5 skip. The most important item is the sixel group (#362, #365, plus #357 and #363). On a sixel terminal, every frame that shows a Block Kit image writes the raw DCS pixel stream at whatever the cursor position is, and nothing erases it. The screen corrupts until you restart. A smaller but real cache bug comes second: the fork caches a `message_changed` echo with an empty subtype and refreshes `synced_at`. Editing a thread broadcast therefore drops it from the cached channel feed, and an edit makes a channel that a reconnect marked stale look fresh again (#348, cache commits only; the fork already has the UI-side changed/edited split). Stacked in commit order, the clean TAKE commits (#342, #339, #340, #364, #357, #362) build, and `go test` passes for `internal/emoji`, `internal/image`, `internal/ui`, `internal/ui/messages/...`, `internal/slack` and `cmd/slk`.

## Take

- **#362: fix: paint block-kit sixel images through the placement pipeline** ([PR](https://github.com/gammons/slk/pull/362)), together with **#365: fix: split the prerendered block image's side channel by protocol** ([PR](https://github.com/gammons/slk/pull/365))
  - What: Block Kit images under sixel stop firing `OnFlush` from View, which wrote the DCS stream at the cursor every frame. They now go through the same SixelRows placement path that file attachments use, keyed by the fetcher cache key. Follow-ups fix where section accessories and legacy-attachment images are placed, and graft halfblock fallbacks into thread renders through a shared `SpliceSixelFallbacks`. #365 closes the same leak on the prerendered fast path that #357 added.
  - Why: screen corruption that only a restart clears, for any sixel user who views a message with a block image. It also stops hashing hundreds of KB of payload per image per frame.
  - Conflict: low. `internal/ui/messages/blockkit/image.go`, `layout`/`attachments.go`, `internal/ui/messages/model.go`, `internal/ui/thread/model.go`, plus the AGENTS.md vocabulary row.
  - Cherry-pick: #362 (`13738d57..42f82bbe`, 8 commits) is clean on its own and on top of #357. #365 (`e61ebe87`) needs #357 first and conflicts in `AGENTS.md`, `blockkit/image.go` and `image_sixel_test.go` when #362 is not already applied. Take #357, #362, #363 and #365 in that order. Done that way, only `internal/ui/messages/model_test.go` conflicts (from #363). Add the new `SpliceSixelFallbacks` row to AGENTS.md's table in the fork's wording.

- **#357: fix: keep the prerender cache across same-protocol reconfigures** ([PR](https://github.com/gammons/slk/pull/357))
  - What: `ConfigurePrerender` drops the fetcher's prerender cache only when the image protocol changes. Before this, it dropped it on every workspace switch.
  - Why: after a switch, every visible image was re-encoded synchronously in View, 140-400 ms per image on sixel, which means a freeze of several seconds. Small fix.
  - Conflict: none (`internal/image/fetcher.go`, `blockkit/image.go`).
  - Cherry-pick: `02178c5a` clean.

- **#363: fix: keep border and gutter on clipped-image fallback rows** ([PR](https://github.com/gammons/slk/pull/363))
  - What: when a partly visible sixel image falls back to halfblock, the fallback is grafted onto the placeholder row (prefix, image, tail) instead of replacing the whole row.
  - Why: clipped images lost the border and gutter, sat 1-6 columns left of their slot, and left stale cells to their right.
  - Conflict: low (`internal/ui/messages/model.go`). The test file conflicts.
  - Cherry-pick: `40957ec0` conflicts in `internal/ui/messages/model_test.go` only (test-placement hunk).

- **#348 (cache commits only): fix(cache): a change echo updates the cached row, not replaces it / leaves the channel's sync state alone** ([PR](https://github.com/gammons/slk/pull/348))
  - What: `8acf8c52` caches a `message_changed` echo through a new `cache.DB.ApplyMessageChange`: a single UPDATE of text plus a `json_patch` of text/files/blocks/attachments into raw_json, with every other column left alone. `698a9bfe` stops a change echo from setting `synced_at` and advancing `latest_synced_ts`.
  - Why: the fork's `OnMessage` sends every echo through `UpsertLiveMessage` with `subtype=""` (`internal/slack/events.go` passes `""` for message_changed). The fork's version keeps reply_count and thread_ts but overwrites subtype, so an edited `thread_broadcast` drops out of the cached channel feed. The echo also refreshes `synced_at`, so a channel that a reconnect marked stale skips its refetch after any edit in it. The rest of the PR (`c7a51e15`, changed/edited split, `ApplyChange` in the panes) is already in the fork in its own form (`OnMessage(..., changed bool, edited *slack.Edited, ...)`, `showBody`, `pendingOps.EditEcho`). Do not take that part.
  - Conflict: high, textually. `cmd/slk/rtm_handler.go`'s cache block and `internal/cache/messages.go` were rewritten in the fork. Port it by hand: in `OnMessage`, when `changed`, call an `ApplyMessageChange`-style update instead of `UpsertLiveMessage`, and skip `SetChannelSyncedAt` and `AdvanceChannelLatestSyncedTS`. Add the new cache method to AGENTS.md's table next to `UpsertLiveMessage`.
  - Cherry-pick: `8acf8c52` conflicts (`rtm_handler.go`, `cache/messages.go`); `698a9bfe` conflicts (`rtm_handler.go`, its test). Its `change_echo_cache_test.go` and the mention test `43862285` are worth bringing along. `43862285`'s and `a328b0d2`'s notify tests use `time.Sleep` ("let a wrongly started notification land", a quiet-period check). Per the no-wall-clock rule, rewrite them to run the handler to completion and then assert.
  - Not recommended from the same PR: `ad50132e` (`entrylist.HoldBottom`, keep the pane at the bottom when a message grows in place). The fork's entrylist already anchors a bottom window across every `SetEntries`, so it probably duplicates that. Check first whether an unfurl landing on a just-sent message pushes rows under the fold in the fork. It conflicts in five files.

- **#364: fix: pin the Activity badge to the workspace's unread-activity total** ([PR](https://github.com/gammons/slk/pull/364))
  - What: the sidebar's Activity badge shows the server's total unread activity: the sum of `activity_v2` from `client.counts` at boot, and from `badge_counts_updated` events, which the fork currently logs as unknown. It is stored per workspace and adjusted by one when you read an item. Before, it counted unread items in the loaded 50-item page.
  - Why: the current badge undercounts and jumps between two wrong numbers when `u` re-fetches a page (upstream saw 3 and 17 where the desktop showed 55). It handles a Slack event that slk drops today.
  - Conflict: low. `cmd/slk/connect.go`, `reconnect_sync.go`, `presence.go`, `internal/slack/client.go` and `events.go`, `internal/ui/reducer_activity.go` and `reducer_workspace.go`. All applied cleanly over the fork's reconnect/session work.
  - Cherry-pick: `297f97a2` clean; builds and tests pass. For the fit pass: `cmd/slk/badge_clock.go` orders a badge event against an in-flight `client.counts` by `time.Now()` stamps. The fork already solves this with the `mark()` pattern in `selfStatus.applyFetched` (`cmd/slk/self_status.go`), so rework it to that rather than adding a second ordering scheme. Also decide whether DMs counted here and in the Unreads pill is acceptable; `activity_v2` includes DMs.

- **#336: fix(ui): o finds links in Block Kit blocks and legacy attachment bodies** ([PR](https://github.com/gammons/slk/pull/336))
  - What: `messages.MessageLinks` also scans top-level section/context blocks and each legacy attachment's pretext, title_link, text, fields and nested blocks. Body text comes from `MessageTextSource` and is used only when the blocks don't carry the body. Results are deduplicated by URL.
  - Why: on Jira-style unfurl notifications (all content in attachment blocks), `o` says "No links in message".
  - Conflict: low (`internal/ui/messages/links.go`; AGENTS.md row for `MessageLinks`).
  - Cherry-pick: `bea6472c` conflicts in `AGENTS.md` and `links_test.go` only. Its tests are examples of a pure function. AGENTS.md wants a property, for example "every URL returned appears in some drawn text source, and no URL repeats", over `proptest.SlackText`.

- **#337: fix(export): convert inline markup inside quotes to CommonMark** ([PR](https://github.com/gammons/slk/pull/337))
  - What: the Markdown export runs quoted lines through `slackMrkdwnToCommonMarkInline` before decoding entities.
  - Why: the fork fixed this on screen (`69e12859`, `9515c481`) but not in the export (`render.go:967`, which still only decodes entities). Quoted links export as raw `<https://…|label>`. The code change is one line.
  - Conflict: low. `internal/ui/messages/render.go` (the Width doc comment hunk; the fork already rewrote that comment) and `render_test.go`.
  - Cherry-pick: `3441863b` conflicts in `render.go` and `render_test.go`. Take only the added line in `SlackMrkdwnToCommonMarkWithUserGroups` and the export test.

- **#339: fix(emoji): keep an emoji that follows a skin-toned one** ([PR](https://github.com/gammons/slk/pull/339))
  - What: in `StripSkinToneFromText`, the regex is replaced with one that matches only toned shortcodes (`::skin-tone-N`, `::skin-tone-N-M`, `_toneN`).
  - Why: in glyph mode and in the export, `:+1::skin-tone-5::tada:` lost the 🎉, and `12:30:+1::skin-tone-5:` left a bare swatch. It is a pure function of its input; a property test would fit, for example "no shortcode without a tone changes, and output never contains `skin-tone`".
  - Conflict: none (`internal/emoji/fuse.go`). It does not touch the fork's VS16/width gate.
  - Cherry-pick: `41ce7ed3 b66152f6 e494ca5e e35b4c22` clean.

- **#340: fix(emoji): draw skin-toned emoji as one image in the right shade** ([PR](https://github.com/gammons/slk/pull/340))
  - What: in image mode, `:+1::skin-tone-2:` is merged into one toned image. The off-by-one between Slack's tone numbers (2-6) and kyokomi's (1-5) is fixed, and so is a VS16 left in composed CDN URLs that made them 404. The generator emits the set of emoji that take a tone.
  - Why: today every toned emoji in image mode is either one shade off or a yellow base plus swatch, and some never load.
  - Conflict: none (`internal/emoji/*`, generated `slacknames_gen.go`).
  - Cherry-pick: `4e89681a fe613cc2` clean on top of #339.

- **#342: fix(image): don't pick kitty for WezTerm under tmux** ([PR](https://github.com/gammons/slk/pull/342))
  - What: drops `wezterm` from the tmux outer-terminal kitty match, so it falls back to halfblock.
  - Why: WezTerm has no kitty unicode-placeholder support, so images under tmux draw at the cursor and show tofu.
  - Conflict: none (`internal/image/capability.go`, wiki).
  - Cherry-pick: `312ded5d` clean.

## Maybe

- **#351: fix(emoji): draw two-person emoji with a tone per person** ([PR](https://github.com/gammons/slk/pull/351))
  - What: `RichTextToMrkdwn` writes `:handshake::skin-tone-2-5:` from a rich_text element's unicode. The generator emits iamcal's per-person image names, which the tokenizer, `URLForShortcode` and reaction pills look up. An unmatched pair draws as the base. Adds `emoji.SlackSkinToneNumber`.
  - Judgement: correct, but niche (two-tone handshakes and couples), and it adds about 300 generated lines. Take it only if you are taking #340 anyway; it builds on it.
  - Conflict: low (`internal/emoji/*`, `internal/ui/messages/blockkit/richtext.go`).
  - Cherry-pick: on top of #339 and #340, `6ab92b79 f88765db 76eac285 9595fd2d` conflict in `AGENTS.md` only. On origin/main alone it conflicts in six emoji files.

- **#341: fix(slackdesktop): read Secret Service via spec-only D-Bus calls** ([PR](https://github.com/gammons/slk/pull/341))
  - What: replaces `r00t2.io/gosecret` with direct godbus calls (OpenSession, SearchItems, GetSecret), so `--add-workspace` works on Secret Service providers other than gnome-keyring (oo7-daemon). It also drops a dependency.
  - Judgement: a real fix, but Linux-only, and about 590 lines (mostly tests). Worth it if the fork's tap or releases have Linux users on non-gnome keyrings. Irrelevant on macOS.
  - Conflict: low. `internal/slackdesktop/password_linux.go`; `go.mod`/`go.sum` conflict with the fork's `x/net` bump, and the Nix `vendorHash` (`5f5b373b`) must be regenerated rather than taken.
  - Cherry-pick: `4b6b0409 5f5b373b` conflicts in `go.mod`, `go.sum`.

- **#347: fix(ui): clicking a [File] line downloads and opens it** ([PR](https://github.com/gammons/slk/pull/347))
  - What: records a `HitFile` rect for `[File]` lines in `RenderMessage`, adds `HitTestFile` to both panes and `contentPane`, and routes a left click to `DownloadFileMsg` (same path as `d`).
  - Judgement: one click downloads and launches an app. That is convenient, but a stray click opens a file where today it selects the message. Decide whether you want that.
  - Conflict: low (`render_message.go`, `reducer_mouse.go`, `content_pane.go`, `entrylist.go`).
  - Cherry-pick: `74c1a41c 623b4b63` conflicts in `internal/ui/messages/render_message.go`.

- **#359: feat: click the compose box to start typing** ([PR](https://github.com/gammons/slk/pull/359))
  - What: panes record their compose region each frame, and a click there enters insert mode through an extracted `beginInsertMode` (same path as `i`).
  - Judgement: small and intuitive. The cost is giving up drag-to-copy that starts in the compose area. It touches the fork's normal-mode key path.
  - Conflict: medium (`internal/ui/mode_normal.go`, `reducer_mouse.go`, AGENTS.md).
  - Cherry-pick: `90d29574` conflicts in those three files.

- **#361: feat: click the thread breadcrumb's channel to go up** ([PR](https://github.com/gammons/slk/pull/361))
  - What: the breadcrumb reports the channel label's cell span, and a click on it closes the thread, like esc.
  - Judgement: small mouse affordance, about 150 lines. Fine if you use the mouse; otherwise nothing gained. Check it against the stacked thread layout.
  - Conflict: low (`internal/ui/thread/breadcrumb.go`).
  - Cherry-pick: `214ec9aa` conflicts in `.gitignore` and `internal/ui/thread/breadcrumb_test.go`.

- **#353: feat(compose): stay_in_insert_after_send option (default off)** ([PR](https://github.com/gammons/slk/pull/353))
  - What: `[compose] stay_in_insert_after_send` keeps the composer focused after Enter. While it is on, app Ctrl shortcuts (Ctrl+P, Ctrl+N, Ctrl+A...) work from an empty composer, and Ctrl+W, Ctrl+H and Ctrl+K keep deleting. Includes two fixes for handing focus to the channel composer when a dialog outlives its thread.
  - Judgement: whether you want this mode at all. The Ctrl-shortcut handoff from an empty composer overlaps directly with the fork's composer keys (ctrl+e, ctrl+x, ctrl+u), and those need a decision about which wins.
  - Conflict: high (`internal/ui/mode_insert.go`, `app.go`, `reducer_channels.go`, `cmd/slk/main.go`, `ui_settings.go`, `internal/config/config.go`, wiki).
  - Cherry-pick: `84bfd01e c5c24c3a 18e927c5 d05df2fa` conflicts in 8 files.

- **#352: feat(ui): `:` command Tab completion, candidates in the status bar, `:help`, `:version`** ([PR](https://github.com/gammons/slk/pull/352))
  - What: Tab completes `:` command names, with candidates shown as pills in the status bar (and marked when clipped). `:help` opens the keybindings overlay; `:version` shows the build identity through `version.Line` / `version.Short`.
  - Judgement: discoverability for `:` commands, about 1100 lines. `:help` overlaps the fork's grouped help overlay, and `:version` may overlap the fork's update hint and `--version`. Taking Tab completion alone is probably the useful part.
  - Conflict: medium (`internal/ui/command.go`, status bar, help-mode handler; wiki `Commands.md` and `Keybindings.md`, and `Commands.md` comes from #350).
  - Cherry-pick: `43b6b306..322f2ec8` (7 commits) conflicts in `wiki/Commands.md` and `wiki/Keybindings.md` only. The `time.After`/timeout matches in its tests (4 lines) need checking against the no-wall-clock rule.

## Skip

- **#349: x removes a link preview from your own message** ([PR](https://github.com/gammons/slk/pull/349)): `chat.deleteAttachment` plus a new `MessageService.RemoveAttachment` port, a chooser, and pending/stale-copy state, about 2000 lines across 34 files for a rare action. Same "large surface for small gain" call as #317. Only `AGENTS.md` conflicts if you ever want it.
- **#360: custom status presets (WFH, vacation) in Ctrl+S** ([PR](https://github.com/gammons/slk/pull/360)): two hardcoded presets plus a self-status segment in the status bar, about 1100 lines. It also truncates with `reflow/truncate`, which AGENTS.md says not to use for styled lines. The segment showing your own custom status is the only part worth lifting by hand.
- **#354: sort conversations alphabetically within sections** ([PR](https://github.com/gammons/slk/pull/354)): replaces Slack's own order inside sections with name order, with no option, and re-blesses 36 goldens. Conflicts with the fork's sidebar (`sortFilteredIdx`, Unreads section) and its goldens.
- **#350: docs: add a Commands wiki page** ([PR](https://github.com/gammons/slk/pull/350)): upstream wiki for its command set; the fork's commands differ (`slk open` and others).
- **#366: add stale workflow and README update** ([PR](https://github.com/gammons/slk/pull/366)): upstream's issue-triage automation.
