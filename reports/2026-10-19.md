# Upstream review 2026-10-19

First review. Range: `e59099c3..07bdfab0` (merge base, PR #314, to upstream/main), 25 non-merge commits in 9 PRs, none already in the fork by patch. Counts: 1 take, 3 maybe, 5 skip. The one to take is #307: the background presence bootstrap reads `wctx.Channels` while the WebSocket event owner appends to it, a data race on every connect and reconnect; the fix snapshots the peer IDs on the event goroutine and cherry-picks cleanly. #331 (block-quote wrapping and inline markup) is already fixed in the fork by its own commits.

## Take

- **#307: fix(presence): snapshot peers before background bootstrap** ([PR](https://github.com/gammons/slk/pull/307))
  - What: `OnConnect` computes `workspacePresenceIDs(h.wsCtx)` on the event owner and passes the slice to `bootstrapPresenceAndDND` / `subscribeWorkspacePresence`, instead of the goroutine reading `wctx.Channels` itself.
  - Why: real data race (goroutine reads the slice while `OnConversationOpened` appends to it); small, confined fix with a regression test.
  - Conflict: none (`cmd/slk/connect.go`, `cmd/slk/presence.go`, `cmd/slk/presence_test.go`).
  - Cherry-pick: `8265ff93 b32d218d` clean; `go test -race -run 'TestOnConnectPresence|TestWorkspacePresence' ./cmd/slk` passes. Note the new `TestOnConnectPresenceSnapshotRacesConversationOpened` uses `select` with `time.After(5s)` as its failure branch, which breaks the fork's no-wall-clock-waits rule; drop the `select` and block on `<-done` when taking it.

## Maybe

- **#317: disable the composer in app DMs that cannot be messaged** ([PR](https://github.com/gammons/slk/pull/317))
  - What: on opening an app/bot DM, probes `conversations.info` (`return_app_home=true`, falling back to `conversations.view`) for `messages_tab_enabled` / `messages_tab_read_only_enabled`; marks such DMs, and any conversation where a send fails with `restricted_action_read_only`, as unsendable for the session. `i`, thread replies and insert mode are blocked with a toast, and the composer is drawn disabled. Adds `ChannelService.MessagingCapability` to `internal/core`.
  - Judgement: a correct user-facing improvement (today you type a message and it bounces), but it is ~1100 lines, adds one extra request per app DM opened, and new App state (`composeDisabled`, `readOnlyChannels`, `unsendableChannels`) checked in key paths. Decide whether the payoff in app DMs is worth that surface. Fails open on probe errors, so the downside is small.
  - Conflict: medium. Touches `internal/ui/mode_insert.go`, `mode_normal.go`, `reducer_send.go`, `reducer_threads.go`, `view_messages.go`, `windows.go`, all areas the fork has reworked (composer keys, optimistic sends, narrow layout); textually it applies cleanly, but the insert/send paths need a read against the fork's `beginSend`/`beginReply` flow.
  - Cherry-pick: `2bb5037e..6c7ab145` (10 commits) clean.

- **#318: copy huddle links for conversations and people** ([PR](https://github.com/gammons/slk/pull/318))
  - What: `H` / `:huddle` copies `https://app.slack.com/huddle/<team>/<channel>` for the conversation on screen; the K profile dialog gets a copy affordance for a peer's active huddle. Replaces slack-go's `GetUserInfo` with a hand-rolled `users.info` POST so it can decode `huddle_state_channel_id`.
  - Judgement: whether a niche feature earns a top-level `H` binding and swapping the users.info call path (all profile fetches, export collection, peer status) for hand-rolled decoding. The URL builder alone is trivial; the users.info rewrite is the risky part.
  - Conflict: high. `internal/ui/keys.go`, `command.go`, help goldens (`dialog_help*.ansi`, fork has the grouped help overlay), `AGENTS.md`, `internal/slack/client.go`, `cmd/slk/export_collect.go`.
  - Cherry-pick: conflicts (first commit `a572c0c1` already conflicts in the files above).

- **#322: perf(scrollbar): build the row pad from a cached style prefix and suffix** ([PR](https://github.com/gammons/slk/pull/322))
  - What: `scrollbar.Overlay` renders one background cell once per call and builds pads by string concatenation instead of a lipgloss render per short row; byte-identical output, with an oracle test over widths 1..200 and benchmarks (about 2x faster, a third of the allocations).
  - Judgement: a clean win, but the absolute saving is ~50-700 us per frame; take it only if pane render cost is something you care about now.
  - Conflict: low. `internal/bubbles/scrollbar/scrollbar.go` conflicts with the fork's wide-rune pad fix (`b630a99b`); resolution is to use `padCells` in the fork's pad line. The oracle test in `pad_test.go` must be checked against the fork's padding behaviour.
  - Cherry-pick: `e4ccae91 247abe9f` conflicts in `scrollbar.go` (one hunk).

## Skip

- **#331: fix(messages): wrap quoted lines under the bar and format their inline markup** ([PR](https://github.com/gammons/slk/pull/331)) - already done in the fork (`69e12859`, `9515c481`); conflicts in `render.go`.
- **#320: perf(bubbles): cache confirmprompt's rendered box** ([PR](https://github.com/gammons/slk/pull/320)) - caches a modal that is rarely open; changes `View` to a pointer receiver; conflicts in `confirmprompt.go`; no felt payoff.
- **#325: test(bubbles): add entrylist and pane scaling benchmarks** ([PR](https://github.com/gammons/slk/pull/325)) - benchmarks only; applies cleanly if ever wanted for perf work.
- **#330: chore: add install task to makefile** ([PR](https://github.com/gammons/slk/pull/330)) - trivial `make install`; conflicts with the fork's `fuzz` target line; add by hand if wanted.
- **#334: docs: describe the automated reviewer in reviewing-prs.md** ([PR](https://github.com/gammons/slk/pull/334)) - upstream's own review process (its bot, Greptile); not relevant to the fork.
