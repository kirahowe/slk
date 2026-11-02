# Upstream review 2026-11-02

Range: `04d0a2d4..4428adf0`. That is 44 upstream units: 43 merged PRs, plus #319, which landed as 23 direct commits on upstream main. There are 157 non-merge commits in all. None is already in the fork by patch, and none is in the ledger. The ledger still says "reviewed through `07bdfab0`" and does not yet record the decisions from the 2026-10-26 report. Counts: 21 take, 4 maybe, 19 skip.

The most important item is #399. slk shows DM peers as away while they are online, for two reasons:
- The socket is opened with `no_query_on_subscribe=1` and slk never sends a `presence_query`.
- Every `UpsertUser` overwrites the cached presence with a placeholder.

Second is #395: `E` on a message with a mention, list or link opens the editor on wire tokens (`<@U…>`) and loses line breaks. Both need hand-ports onto the fork's code (wsSession, `switchLists`, `enterEdit`/`translateMentions`).

Many of the takes are one-line or very small, and four of them pick cleanly together: #383, #385, #386 and #381. A trial merge found no common merge base between origin/main and upstream/main, so 3-way cherry-picks sometimes fall back to plain patch application.

## Take

- **#399: fix(presence): DM peers shown away while they are online** ([PR](https://github.com/gammons/slk/pull/399))
  - What: `UpsertUser` no longer overwrites `presence` on conflict. A workspace switch rereads DM presence from the cache (`withCachedPresence`). A `presence_query` is sent after `presence_sub`.
  - Why: a peer's reply re-upserts their profile and resets them to away. Peers already online before connect are never reported, because the socket uses `no_query_on_subscribe=1`.
  - Conflict: high in `cmd/slk/main.go`: the fork uses `switchLists(wctx, statuses)`, so put `withCachedPresence` inside `switchLists` in `cmd/slk/workspace.go`. High in `internal/slack/client.go`: upstream's `QueryPresence` uses `c.wsMu`/`c.wsConn`, which the fork replaced with `wsSession`.
  - Cherry-pick: `18f0a57a` (cache) is clean, and `internal/cache` and `cmd/slk` pass. `4d375891` conflicts in `cmd/slk/main.go`. `752fa085` conflicts in `cmd/slk/presence.go` and does not build against the fork's client. `6155923a` conflicts in AGENTS.md.
  - Fit:
    - Rewrite `QueryPresence` over `(*wsSession).writeJSON` with the `maxClientFrame` limit.
    - Query only the `ids[:n]` prefix that `SubscribePresence` reports it sent. Size the frame the way `presenceSubFit` does, since the gateway closes the socket on an oversized frame.
    - Add the AGENTS.md row in the fork's table, next to the WebSocket write row.

- **#395: fix(compose): edit a message as it was typed (mentions, lists, links)** ([PR](https://github.com/gammons/slk/pull/395))
  - What: `E` seeds the composer from the message's rich_text blocks rendered as typed markdown (`messages.EditSource`, `blockkit.RichTextToMarkdown`). It also turns `<@U…>`/`<#C…>` back into `@name`/`#name` (`compose.MentionsForEdit`).
  - Why: the fork seeds the editor with raw text through `beginEditOfSelected` → `enterEdit`. Editing any message with a mention, list or link shows wire tokens. This is a common action.
  - Conflict: high in `internal/ui/app.go` and `internal/ui/compose/model.go`. The fork split `beginEditOfSelected`/`enterEdit` and has the pure `compose.translateMentions` where upstream refactored `sendMentions`.
  - Cherry-pick: `11badcf5 8770792b e779dd25 15ba6a6f 1155b325 6eca0380 c1415834` conflict in those two files. The other 8 files apply. Hand-port.
  - Fit:
    - Write `MentionsForEdit` as a pure function of the same maps, beside `translateMentions`.
    - Add a rapid round-trip property against `mrkdwn.Convert`. Today `RichTextToMarkdown`, `EditSource` and `MentionsForEdit` have example tests only.
    - Check that `reopenEdit` and `messages.Body`/`TypedBody` agree with the new seed text.
    - Merge upstream's two AGENTS.md rows into the fork's table.

- **#415: fix(mentions): resolve user names in message content** ([PR](https://github.com/gammons/slk/pull/415))
  - What: new `cmd/slk/resolveMessageMentions`. It collects `<@U…>` mentions from every visible text part (body, section and context blocks, legacy attachment pretext, text, fields and blocks), fills names from the cache, and queues misses on the source workspace's `UserResolver`. It runs on history fetches, cache reads, thread replies and live events.
  - Why: a mention of a user slk has never resolved shows as `@U123` permanently. The fork has nothing for this; `MentionedNames` only feeds notifications.
  - Conflict: low, `wiki/Features.md` only. It adds one line to each of the three duplicated fetch functions in `cmd/slk/history.go`.
  - Cherry-pick: `eb85e7f5` conflicts only in `wiki/Features.md` (keep ours). Build, vet, `cmd/slk` and `internal/ui` all pass.
  - Fit: `cmd/slk/message_mentions_test.go` (two places) uses `select { case <-watch.done: case <-time.After(2s) }`. Make each a bare `<-watch.done`. Keep the AGENTS.md row it adds.

- **#316: fix: copy, export and open links from a forwarded message's shared text** ([PR](https://github.com/gammons/slk/pull/316))
  - What: new `messages.MessageCopyText`. It returns the body, or for a forwarded message, falls back to the shared attachment's text (comment, then shared text). It is used by `y`, the markdown export, export index snippets, the export mention scan and `MessageLinks`.
  - Why: `y` on a forwarded message says "Message has no text", and the export loses forwarded content.
  - Conflict: low: AGENTS.md and `internal/ui/messages/links.go`. The fork's `MessageLinks` is simpler, so take only `out := ExtractLinks(MessageCopyText(msg))`. If #336 from the last report is taken, reconcile with it.
  - Cherry-pick: `a9860f03 54772403 b5cc3d78 4b3cdb4b 522b03bd 2bb660a7 bc1dc26b`, one at a time. AGENTS.md conflicts in four of them and `links.go` in two, all resolved as above. Build passes, and `messages`, `export`, `ui` and `cmd/slk` pass.
  - Fit: `MessageCopyText` is pure with example tests only. Add a property, for example: the result contains `MessageTextSource(msg)` whenever that is non-empty. Add its row in the fork's table.

- **#417: fix(ui): keep avatars intact when author headers wrap** ([PR](https://github.com/gammons/slk/pull/417))
  - What: `RenderMessage` wraps the author header to `HeaderBudget` before the avatar gutter is added. The timestamp and edited mark are kept whole, and the real header height feeds the image and hit-rect rows.
  - Why: in a narrow pane a long display name splits the avatar image and moves image and reaction hit rects by a row. The fork's narrow-terminal layout makes this more likely.
  - Conflict: low, AGENTS.md only.
  - Cherry-pick: `8a4d9706` conflicts only in AGENTS.md. Build passes, and `internal/ui` passes with goldens unchanged. One `internal/ui/messages` test (`TestWrapRows_WrappedHeaderKeepsActionRowsAligned`) does not compile because it uses `WrappedRows.LinkHits` and `entrylist.HitFile` from upstream #358. Drop it or adapt it.
  - Fit: the header wrap is close to pure. Add a property such as "every header line fits `HeaderBudget`, and the timestamp stays whole when it fits".

- **#409: fix(messages): keep the theme's text colour on Block Kit lines, attachments and after inline emoji** ([PR](https://github.com/gammons/slk/pull/409))
  - What: re-emits the theme foreground at the start of each Block Kit line, after the legacy-attachment stripe, and after an inline kitty emoji (which ends in `\e[39m`).
  - Why: on themes whose text colour is not the terminal default, bullets, attachment titles and text after a custom emoji are drawn in the terminal colour. On some themes that is unreadable.
  - Conflict: low, goldens only.
  - Cherry-pick: `d34dcd32 42ed9a0a`. The code merges cleanly; `narrow.ansi`, `thread_open.ansi` and `thread_parent_selected.ansi` conflict. Keep the fork's goldens and run `go test ./internal/ui -run TestGolden -update`: 12 goldens change, 29 lines. Then `ui`, `messages` and `blockkit` pass.
  - Fit: re-bless rather than take upstream's goldens. This does not duplicate `ReapplyAfterResets`, which handles resets, not a trailing `39m`.

- **#416: fix(sidebar): load starred conversations from the boot snapshot** ([PR](https://github.com/gammons/slk/pull/416))
  - What: Starred membership comes from `client.userBoot`'s `starred` list instead of `stars.list`, skipping unknown entry shapes. Startup reuses the boot result it already has; a reconnect fetches a fresh userBoot.
  - Why: `stars.list` is restricted or empty on Enterprise Grid, so the Starred section is blank there. It also saves one RPC at startup. Cost: on reconnect a full userBoot runs on the event owner, bounded by the existing 30s ctx and the SectionStore debounce. The payoff depends on whether any of Kira's workspaces are on Grid.
  - Conflict: low. `cmd/slk/connect.go` conflicts on the OnConnect comment and on upstream's `sectionRefreshAt` gate (from #278); keep the fork's side or bring #278's gate.
  - Cherry-pick: `df44f64c` conflicts only in `cmd/slk/connect.go`. After resolving, build passes but vet fails: `starred_boot_test.go` appends to `wctx.Channels` as a field and sets `sectionRefreshAt`.
  - Fit: seed the test through the owner slice plus `publishChannels()` and hand the UI `wctx.Channels()`. Its `time.Sleep(31s)` runs inside a `synctest` bubble, which the fork allows.

- **#278: fix(starred): debounce reconnect recovery, include recovered peers in presence, keep newer finder visits** ([PR](https://github.com/gammons/slk/pull/278))
  - What: three follow-ups to starred people, which the fork already has:
    - `OnConnect` runs the presence bootstrap after starred-row recovery, so recovered starred DMs get presence.
    - The section refresh and starred reconciliation are debounced to once per 30s across connection flaps.
    - `channelfinder.Upsert` keeps the newer `LastVisited`.
  - Why: the fork's `OnConnect` has the same ordering bug, because presence runs through `wctx.selfFetch` before the section refresh. The other two are small correctness fixes.
  - Conflict: high in `cmd/slk/connect.go` (the fork's `selfFetch`/`latestRunner`) and in `cmd/slk/starred_deferred_resolution_test.go` (the fork's unexported owner slices).
  - Cherry-pick: `754bf2c5 35d56891 81192e02 e90c2355 9419b06d f7234aee ec5e7976 2e0f8cbf` conflict in `connect.go`. Taken as a whole diff, only those two files conflict; `rtm_handler.go`, `channelfinder` and the other tests apply.
  - Fit:
    - The new tests use `time.After(5s)` three times; make them blocking receives.
    - The debounce reads `time.Now`/`time.Since` on `sectionRefreshAt`; inject `now`.
    - Tests go through `FinderItems()`/`Channels()`.
    - The moved presence fetch stays inside `selfFetch.Run`.
    - Take it with or before #416, which assumes its gate.

- **#398: feat(thread): p goes to the open thread's message in its channel** ([PR](https://github.com/gammons/slk/pull/398))
  - What: with a thread open, `p` closes the thread and jumps to its parent message in the channel through `applyLocation`. It records the thread as the back-jump target, so `''` returns. It also extracts `threadPanelLocation()` from `currentLocation`.
  - Why: about 50 lines plus a test, built on the fork's own mark and back-jump machinery. `p` is unbound in the fork.
  - Conflict: low. `internal/ui/keys.go` conflicts on the fork's grouped-help `help:"…"` tags, and so do the two help goldens.
  - Cherry-pick: `1c61c14d 3955b5c4` conflict in `keys.go`, `dialog_help.ansi` and `dialog_help_narrow.ansi`. With keys.go resolved (`JumpToParent key.Binding \`help:"Threads"\``) and the fork's goldens kept, build passes. One test fails: `TestEveryBoundKeyReachesAHandler/JumpToParent/p` ("p did nothing", because no thread is open in that fixture).
  - Fit: add `"JumpToParent/p": "only with a thread open; none is"` to `keyMapReachExempt` in `internal/ui/keymap_reach_test.go`, or open a thread in that case. Re-bless the help goldens if the overlay lists the new key.

- **#378: fix(ui): accept non-ASCII runes in the command line and finders** ([PR](https://github.com/gammons/slk/pull/378)), together with **#393: fix(ui): accept non-ASCII runes in the theme, presence, reaction and help searches** ([PR](https://github.com/gammons/slk/pull/393))
  - What: the command line, channel finder, workspace finder, new-message picker, help search, presence menu, reaction picker and theme switcher accept any single printable rune. Backspace removes a whole rune.
  - Why: accented and CJK channel or user names cannot be typed today, and backspace can cut a UTF-8 sequence in half.
  - Conflict: low. `internal/ui/mode_command.go` for #378, because the fork writes `a.cmdline` directly instead of `setCmdline`. #393 has none.
  - Cherry-pick: `94bb0bc8` (#378) conflicts only in `mode_command.go`. After a hand-port it builds, and the finder and picker packages pass. `c8025253` (#393) is clean, and its four packages pass. The fork's `TestCommandMode_NonASCIIRuneIsDropped` pins the old bug; re-pin it.
  - Fit: the two PRs together leave eight inline copies of the same printable-rune check and rune backspace. Replace them with one shared helper, listed in AGENTS.md, with a property test (typing a rune then backspace restores the query).

- **#375: fix(messages): show app link chips (attachment_mention) by their title** ([PR](https://github.com/gammons/slk/pull/375))
  - What: rich_text `attachment_mention` elements (Linear and other Work Object chips) render as `<url|title>`. Link labels are entity-decoded in the renderer and in `ExtractLinks`.
  - Why: a message sent from the web client with a chip currently loses the link entirely, and labels show `&amp;`.
  - Conflict: low, `internal/ui/messages/blockkit/richtext_test.go` (both sides added tests).
  - Cherry-pick: `7545bc57 68b4a0a7` conflict only in that test file. Resolved, build passes and `blockkit` passes. Two `messages` tests fail, `TestRenderMessage_AttachmentMentionShowsTitle` and `TestMessageLinks_AttachmentMentionTitleUnescaped`, because they depend on upstream's block-scanning `MessageLinks` (#336 in the last report). Take #336 first or adapt them.
  - Fit: label escape and decode are pure; add a property that a label survives escape then decode.

- **#402: fix(messages): keep links to other Slack messages (message_mention) in the body** ([PR](https://github.com/gammons/slk/pull/402))
  - What: rich_text `message_mention` elements (a link to another Slack message) are rebuilt as `<url>` links instead of being dropped.
  - Why: the fork draws the body from the rich_text block. slack-go delivers this element as `RichTextSectionUnknownElement`, which `inlineToMrkdwn` drops, so pasted message permalinks disappear from the rendered message.
  - Conflict: high for a cherry-pick. It sits on #375 and on `richtext_markdown.go` (from #395), and the fork has neither.
  - Cherry-pick: `1436479e` conflicts in `richtext.go`, and the two `richtext_markdown*` files are missing. Port the small `RichTextSectionUnknownElement` case by hand, after #375.
  - Fit: reuse the fork's `linkToMrkdwn`. Bring `testdata/message_mention.json`.

- **#400: fix(preview): open images without a thumbnail (SVG) in the system viewer** ([PR](https://github.com/gammons/slk/pull/400))
  - What: `O`/`v` on an image with no thumbnail (an SVG) downloads it and opens it in the OS viewer through the existing `DownloadFileMsg` path.
  - Why: the keys do nothing on SVG uploads today. It is about 30 lines and adds no new I/O in `internal/ui`.
  - Conflict: low. A comment-adjacent hunk in `internal/ui/app.go` (beside `submitEdit`'s doc) and `wiki/Keybindings.md`.
  - Cherry-pick: `4e050f4d`; resolve `app.go`. Build passes. Fork-only tests fail because their image fixtures have no `Thumbs`: `TestEveryBoundKeyReachesAHandler/OpenPreview/{O,v}` and `TestMessageKeysSkipTheHiddenChannelPane/{O,v}`.
  - Fit: give those fixtures' image attachment a thumbnail.

- **#390: fix(messages): collect warm emoji uploads from Block Kit text** ([PR](https://github.com/gammons/slk/pull/390))
  - What: `BlockkitContext` passes the message's `BlockFlushes` collector to RenderText.
  - Why: kitty custom emoji in Block Kit text draw placeholders whose image uploads are never flushed. The fork still has `EmojiFlushes: nil`.
  - Conflict: low, `blockkit_integration_test.go` imports.
  - Cherry-pick: `abeb9395` conflicts on that test's imports only (drop the unused `encoding/json`). Then `internal/ui/messages` passes.
  - Fit: update the `messages.BlockkitContext` row in AGENTS.md for the new parameter.

- **#377: fix(config): make appearance.show_avatars work** ([PR](https://github.com/gammons/slk/pull/377))
  - What: `ShowAvatars` defaults to true, and `main.go` wires the avatar service only when it is set.
  - Why: the setting is read but ignored. With the default true, behaviour does not change unless you set it.
  - Conflict: low, an end-of-file clash in `internal/config/config_test.go`.
  - Cherry-pick: `d1845c43` conflicts only there. Resolved, `internal/config` and `cmd/slk` pass.
  - Fit: rewrite the test with `loadConfigString(t, body)` instead of TempDir and WriteFile.

- **#385: fix(membership): record a failed member write so the backoff holds** ([PR](https://github.com/gammons/slk/pull/385))
  - What: when `ReplaceChannelMembers` fails, `lastFailed` is set so `EnsureFresh` does not refetch at once.
  - Why: today a SQLite write that keeps failing causes a fetch storm against Slack.
  - Conflict: none.
  - Cherry-pick: `504a1640` clean; the membership tests pass.

- **#386: fix(compose): bump the render version in RefreshStyles** ([PR](https://github.com/gammons/slk/pull/386))
  - What: `RefreshStyles` calls `m.dirty()`.
  - Why: after a theme change the composer serves a stale cached frame in the old theme. One line.
  - Conflict: none.
  - Cherry-pick: `42d60b76` clean; compose tests pass.

- **#383: fix(ui): let shift+enter and shift+esc submit and cancel search** ([PR](https://github.com/gammons/slk/pull/383))
  - What: the `/` search prompt matches on the key code, so shift+enter submits and shift+esc cancels.
  - Why: real input bug; the fork has the same `key.Matches` code in `internal/ui/mode_search.go`.
  - Conflict: none.
  - Cherry-pick: `986223a1` clean; search tests pass.

- **#381: fix(ui): clear the link picker's kind when a link is committed** ([PR](https://github.com/gammons/slk/pull/381))
  - What: `pickerKind` and `pickerFiles` are cleared on every commit, not only when the committed link is a file.
  - Why: a stale "files" kind can send the next picker down the wrong path.
  - Conflict: none.
  - Cherry-pick: `91101a20` clean. #383, #385, #386 and #381 pick together cleanly, and `go build ./...` passes with all four.

- **#394: test(ui): pin every Mode to a handler with a modeCount sentinel** ([PR](https://github.com/gammons/slk/pull/394))
  - What: adds a `modeCount` sentinel to the Mode iota and loops over it in `TestEveryModeHasAHandler`.
  - Why: test only and low priority. It replaces a hand-maintained list, which the fork's `ModeCanvas` already shows can drift. This is the AGENTS.md "replace a comment or list with a check" rule.
  - Conflict: low, `internal/ui/mode.go` and `modekeys_test.go` (the `ModeCanvas` line).
  - Cherry-pick: `3d91692e` conflicts in both files; put `modeCount` after `ModeCanvas`.

## Maybe

- **#319: render system messages as centred notices and show who started a huddle** ([PR](https://github.com/gammons/slk/pull/319)), 23 direct commits `681db9e9..4428adf0`
  - What:
    - Join and leave events (`channel_join/leave`, `group_join/leave`) are forwarded live and drawn as one muted notice line in both panes. They are kept out of notifications, the unread mark and mention counting.
    - Later commits do the same for `huddle_thread` and `tabbed_canvas_updated`, centre the notices like date separators, refuse to open threads on reply-less notices, and add spacing, search highlighting and reactions on notices.
    - The bulk is huddle work: room capture through a slack-go response tap (`internal/slack/huddle_room.go`), core and cache plumbing (a `json_set` on `raw_json` in `UpsertMessage`), and "X started a huddle, lasted N min". About 3300 added lines in all.
  - Judgement: the join/leave subset (`681db9e9`, `6a009136`, and optionally `7ddfeb0a` minus the huddle wording, since canvas-update notices suit the fork's canvas viewer) fixes a real fork bug. The fork's `internal/slack/events.go` subtype switch drops these events live, and from history they draw as Slackbot chat bubbles. An invitation join names the inviter, so it can also get the mention gutter. Skip the huddle commits (`f6b8dbbe 779fc3ce 5b240344 46682d69 60d9e32a 38d72f34 c9990638 3f98ae82 01628e9c`): huddles are not used, and they touch `internal/slack/client.go` and the cache. Decide whether the centred, bold divider style (`507a51e2`) and the "Can't reply to a system message" thread gate (`27435bd3`, `app.go` and `reducer_links.go`) are wanted beyond a plain muted line.
  - Conflict: high for the series: AGENTS.md, `cmd/slk/rtm_handler.go`, `internal/ui/messages/render_message.go`, `internal/slack/events.go`. Each later commit in the subset conflicts once the earlier ones are hand-applied.
  - Cherry-pick (subset): `681db9e9` conflicts in AGENTS.md, `rtm_handler.go` and `render_message.go`, all mechanical:
    - `rtm_handler.go`: add `&& !messages.IsSystemNoticeSubtype(subtype)` to the fork's notify and `shouldMarkChannel` gates.
    - `render_message.go`: put the notice branch ahead of `mention.Classify`.
    - `6a009136` conflicts in `cmd/slk/event_handler_test.go`. Keep the fork's file, append the join test, and add one extra `nil` for the `*slack.Edited` parameter.
    - After that, `cmd/slk`, `messages`, `thread` and `internal/slack` pass.
  - Fit: add an AGENTS.md row for `messages.IsSystemNoticeSubtype`. The subset has no wall-clock waits.

- **#358: feat: click links and file lines with the mouse** ([PR](https://github.com/gammons/slk/pull/358))
  - What: finds OSC 8 spans in the wrapped rows (`messages/linkhits.go`, `ScanOsc8Spans`) and makes them click targets. Clicking a `[File]` line downloads the file. Clicks under the image preview are blocked. About 640 lines.
  - Judgement: is mouse link-clicking worth 640 lines when `o` and the terminal's own Shift+Click work? It depends on upstream's `downloadFileAt` file-click hit test, which the fork lacks (#347, MAYBE last week), and links that wrap are not clickable.
  - Conflict: high: AGENTS.md, `bubbles/entrylist/entrylist.go`, `content_pane.go`, `messages/model.go`, `reducer_mouse.go`.
  - Fit if taken:
    - It truncates with `truncate.StringWithTail`, which AGENTS.md says to replace with `ansi.Truncate`.
    - `ScanOsc8Spans` is a byte parser with example tests only and needs a Fuzz target.

- **#389: feat(emoji): rank frecent emoji first in the reaction picker and autocomplete** ([PR](https://github.com/gammons/slk/pull/389))
  - What: ranks frecent emoji first in the `:` autocomplete as well as the picker. It records emoji used in sent messages, replies and captions, and counts toned uses toward the base emoji. About 830 lines.
  - Judgement: is a modest ranking improvement worth this much code?
  - Fit if taken:
    - It hooks the old `reduceSendMessage`/`SendThreadReplyMsg` paths, which in the fork would move to `beginSend`/`beginReply`.
    - It calls `RecordFrecent` (a SQLite write behind a port) on the Update goroutine.
    - `emoji.Match`, `UsedInText` and `Rank` are pure with example tests only.
  - Conflict: high: AGENTS.md, `app.go`, `reducer_send.go`, `reducer_threads.go`, `reactionpicker/click_test.go`.

- **#376: feat(slack): send links to Work Objects as chips, like the web client** ([PR](https://github.com/gammons/slk/pull/376))
  - What: about 1500 lines, 470 of them production code. Each send calls `chat.unfurlLink` for every bare link and waits up to 3s for an `unfurl_preview_updated` frame. Links whose app publishes a Work Object (Linear and similar) are then posted as `attachment_mention` chips through a hand-built `chat.postMessage`.
  - Judgement: worth a large protocol surface and up to 3s extra latency on every send with a link? Only if Kira's workspaces use Linear-style apps. Receiving chips is handled by #375 either way.
  - Conflict: high: `internal/slack/client.go` (the fork's wsSession dispatch), and `internal/ui/selfsend.go`, which the fork deleted for `pendingOps`, so the second commit does not apply.

## Skip

- **#401: show a hidden DM once it turns unread** ([PR](https://github.com/gammons/slk/pull/401)): the fork already does this through its Unreads section's `rebuildFilter`. Upstream's main test passes on the fork unchanged.
- **#379: keep the scroll position inside the parent on resize** ([PR](https://github.com/gammons/slk/pull/379)): the fork's entrylist already anchors by entry ID, and the thread parent has one. High conflict.
- **#380: draw the thread header separator with ─** ([PR](https://github.com/gammons/slk/pull/380)): the fork already draws `─`.
- **#405: "Unread mentions" pills for mentions scrolled out of view** ([PR](https://github.com/gammons/slk/pull/405)): the fork's Unreads section and status-bar pill already do this job.
- **#407: optional Slack-like sidebar (indent, look, Active-only filter, private glyph, mouse resize)** ([PR](https://github.com/gammons/slk/pull/407)): five cosmetic options in about 540 lines, including a punctuation-ignoring re-sort. It is stacked on upstream sidebar options the fork lacks and conflicts heavily with the Unreads section.
- **#404: optional Slack-like names, selected-row pill and mention badges** ([PR](https://github.com/gammons/slk/pull/404)): cosmetic theme roles stacked on upstream sidebar options the fork lacks.
- **#406: optional Slack-style reply row with replier avatars** ([PR](https://github.com/gammons/slk/pull/406)): opt-in cosmetic, kitty-only, that changes `core` types and ports. High conflict in `cmd/slk/history.go` and `messages/*`.
- **#391: optional author avatars in the thread pane** ([PR](https://github.com/gammons/slk/pull/391)): tiny opt-in cosmetic of low value.
- **#321: call chrome.Window directly in the dialogs** ([PR](https://github.com/gammons/slk/pull/321)): refactor churn. It conflicts with the grouped help overlay (`internal/ui/help/model.go`).
- **#387: correct the normal- and insert-mode header comments** ([PR](https://github.com/gammons/slk/pull/387)): the comments describe upstream-only features (`stay_in_insert_after_send`, `x` remove preview).
- **#326: whole-App benchmarks with a dialog open** ([PR](https://github.com/gammons/slk/pull/326)): benchmarks only, like the declined #325.
- **#324: chrome List and Box benchmarks** ([PR](https://github.com/gammons/slk/pull/324)): benchmarks only.
- **#323: ansi.ReapplyAfterResets benchmarks** ([PR](https://github.com/gammons/slk/pull/323)): benchmarks only.
- **#367: pin dependencies** ([PR](https://github.com/gammons/slk/pull/367)): upstream Renovate SHA-pinning of upstream workflows, including a `stale.yml` the fork does not have. Pin the fork's actions by hand if you want that hardening.
- **#368: update github actions** ([PR](https://github.com/gammons/slk/pull/368)): upstream Renovate bump (install-nix-action, renovate action).
- **#369: actions/checkout v7** ([PR](https://github.com/gammons/slk/pull/369)): upstream Renovate bump.
- **#370: actions/setup-go v7** ([PR](https://github.com/gammons/slk/pull/370)): upstream Renovate bump.
- **#371: actions/upload-artifact v7** ([PR](https://github.com/gammons/slk/pull/371)): upstream Renovate bump.
- **#372: goreleaser-action v7** ([PR](https://github.com/gammons/slk/pull/372)): upstream Renovate bump; the fork has its own release config.
