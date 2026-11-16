# Upstream review 2026-11-16

Reviewed upstream `13f3bd0f..7d4f9b79` (the ledger's reviewed-through commit is later than the last report's `cd6ca030`). That covers 20 merged PRs and 77 non-merge commits. None of them were already in the fork, and none were in the ledger. The verdicts are 2 take, 7 maybe and 11 skip. The most important item is the read-mark fix from #459 (`7ddadce9`, `9dc0507d`). The fork has the same bug in its narrow stacked layout: a message that arrives in the channel or thread pane while the other pane is in front gets marked read. Slack's cursor moves and the unread badge is lost, even though the user never saw the message. That fix needs to be reworked onto the fork's held-mark design; it does not cherry-pick.

## Take

- **[#459](https://github.com/gammons/slk/pull/459): f toggles full screen for the focused pane (take in part: `7ddadce9`, `9dc0507d`)**
  - What: `contentPanesDrawn()` reports which content panes the current frame draws. A message arriving in an open pane that is not drawn now stages no read mark and re-reads read state. `dropMarksForHiddenPanes` also drops marks staged while the terminal was blurred if the pane is hidden when focus returns.
  - Why: this is a correctness bug in the fork. In `reduceNewMessage`, `threadShown` is `inOpenThreadPanel && CursorAtEnd()`, and the channel branch only checks `CursorAtEnd()`. In the stacked layout, the pane that is not in front still has its cursor at the end, so the arrival is staged with `shown=true` and marked read. Upstream's commit message says this predates full screen.
  - Rework: don't drop the mark the way upstream does. Stage it held (`shown=false`) when the pane is not drawn, so `releaseShownMarks` issues it once the pane comes to the front. Check that `Shows(ts)` is false for an undrawn pane; entrylist's last frame may be stale there. The fork's `Compute` has no `compact` argument and there is no `a.fullscreen`, so `frameInto`/`contentPanesDrawn` reduce to `Compute(... a.threadInFront())` into a scratch `panelLayout`.
  - The rest of the PR (`c6a32443` and others, the `f` full-screen mode, about 600 lines) is a product call: the fork's narrow layout already stacks panes. Lean skip.
  - Conflict: high. `internal/ui/app.go`, `internal/ui/reducer_send.go` and `internal/ui/reducer_focus_test.go` diverge from the fork's held marks (`recordChannelMark(…, shown)`).
  - Cherry-pick: conflicts (`7ddadce9` alone). Tests use no wall-clock waits.

- **[#458](https://github.com/gammons/slk/pull/458): leave the current user out of group DM names**
  - What: `slackfmt.FormatMPDMName(name, self, lookup)` drops the user's own handle (auth.test `user`, new `Client.UserName()` and `WorkspaceContext.UserHandle`). Every group DM then reads as the other people, unless the user is the only one left.
  - Why: a small, clear UX fix. The user is in every group DM they can see, so listing them is noise in the sidebar, the finder and the compose placeholder. It fits the fork's recent `5b114641` (name a DM without `#` in the compose box).
  - Conflict: low. `cmd/slk/workspace.go` conflicts only in the struct-field block, because the fork made `channels`/`finderItems` unexported with snapshots. Add the `UserHandle` field by hand. `AGENTS.md` row for `FormatMPDMName` needs the new signature.
  - Cherry-pick: conflicts (trivial, `cmd/slk/workspace.go`). `FormatMPDMName` is pure but only has example tests. AGENTS.md wants a rapid property here, for example: self never appears unless it is the only handle, and the order of the others is kept.

## Maybe

- **[#419](https://github.com/gammons/slk/pull/419): resolve channel names outside the sidebar**
  - What: adds a `channelResolver` per workspace (cache first, then a rate-limited `conversations.info` with a 30s cooldown per failed ID). It names bare `<#C…>` mentions of channels the user has not joined, in fetched, cached, live and thread messages, without adding sidebar rows.
  - Judgement: today such mentions show as raw IDs. Is that worth about 1100 lines (450 of them `cmd/slk` code and ports) and a second map type layered on `userNameStore`?
  - Fork fit: it should follow the fork's resolver rules: the `isMissing`/`markMissing` ask-once set rather than a timed cooldown, and an injected clock. `channel_mentions_test.go` has a `select` with `time.After(2 * time.Second)` and a `time.Sleep(25ms)`, which the no-wall-clock rule forbids.
  - Conflict: high. `cmd/slk/connect.go`, `message_mentions.go`, `internal/ui/app.go` (`SetChannels`), `reducer_workspace.go`, AGENTS.md.
- **[#452](https://github.com/gammons/slk/pull/452): show "Downloading X…" while a file downloads; queue failures**
  - What: `d` now shows a progress toast and a result toast. Overlapping downloads and failures are queued so one does not overwrite another.
  - Judgement: a useful small cue, but 80 lines of queueing state on `App` (`pendingDownloads`, `pendingDownloadFailures`, its own clear message) for a corner case.
  - Fork fit: if taken, the simple progress-then-result part is probably enough. Upstream's `toastTick` is `tea.Tick`; in the fork, use `tick`.
  - Conflict: low (`internal/ui/msgs.go`).
- **[#454](https://github.com/gammons/slk/pull/454): sidebar `start_collapsed`, `show_unread_in_collapsed`; reveal the opened channel without expanding its section**
  - What: two config options, plus a behaviour change: opening a channel in a collapsed section shows it alone under the header instead of expanding the section. `8788dfc5` keeps the cursor on the section header when its row is hidden by a collapse, instead of jumping to Threads.
  - Judgement: do you want the reveal-without-expand behaviour? The two options are sidebar option creep, like #407. The cursor fallback alone is a small nicety.
  - Conflict: high (`internal/ui/sidebar/model.go`; `8788dfc5` depends on `411bd8c9`).
- **[#469](https://github.com/gammons/slk/pull/469): hide the workspace rail when only one workspace is configured**
  - What: the rail gets zero width with a single workspace and keeps its width until workspaces load.
  - Judgement: only matters if you run slk with one workspace. It saves 6 columns, which helps the narrow layout.
  - Conflict: medium (`internal/ui/app.go`, `internal/ui/windows.go`, near the fork's narrow-layout code). Cherry-pick: conflicts.
- **[#463](https://github.com/gammons/slk/pull/463): space thread replies like channel messages; thread avatars on by default**
  - What: `7007853e` replaces the gray rule between thread replies with a blank row (a 10-line change in `thread/model.go`). `2a93b360` defaults `thread_avatars` to true; that part does not apply, because the fork has no `thread_avatars` option (#391 declined).
  - Judgement: purely visual.
  - Conflict: low in code; the goldens need re-blessing.
  - Cherry-pick: conflicts (goldens only).
- **[#468](https://github.com/gammons/slk/pull/468): insert-mode highlight changes**
  - What: hides the selected-message highlight while composing, uses the compose box's left bar as the insert-mode cue, and switches the selection tints.
  - Judgement: an upstream taste change across 36 files and many goldens, and it touches compose styling the fork's draft preview depends on. Lean skip unless the current insert-mode look bothers you.
  - Conflict: high (`internal/ui/compose/model.go`, `messages/model.go`, `thread/model.go`, `styles/tint.go`, draft-preview goldens).
- **[#427](https://github.com/gammons/slk/pull/427): sidebar section-header, DM status and finder status emoji as images**
  - What: in image emoji mode, sidebar section emoji, DM status emoji, the finder and the DM header draw images instead of glyphs. It also fixes section headers (one line, bold when selected, one space after the emoji).
  - Judgement: cosmetic, about 2900 lines. The author-status part overlaps what the fork took with #418 (`AuthorStatusSuffix` with opts). Lean skip.
  - Fork fit: if taken, check against the fork's VS16 width gate (#301) in the sidebar.
  - Conflict: high (`internal/ui/sidebar`, `messages`, `channelfinder`, `bubbles/ansi`).

## Skip

- **[#455](https://github.com/gammons/slk/pull/455): emoji and kitty images on WezTerm via emulated placement**: about 2200 lines for one terminal; take only if you use WezTerm (compare #443, declined).
- **[#460](https://github.com/gammons/slk/pull/460): `transparent_background` theme option**: a cosmetic option across 48 files.
- **[#465](https://github.com/gammons/slk/pull/465): reload custom themes on SIGUSR1**: for theme authors running a palette generator; no payoff for the fork.
- **[#471](https://github.com/gammons/slk/pull/471): unread dots in `:ws`, h/l into the workspace rail**: navigation changes to a rail the fork does not use this way; key conflicts possible.
- **[#472](https://github.com/gammons/slk/pull/472): `workspace_picker_vim_keys`, `enter_keeps_focus` options**: config option sprawl.
- **[#461](https://github.com/gammons/slk/pull/461): detect the Nash terminal as kitty**: sets `KITTY_WINDOW_ID` from two `init()` functions, which is a hack, for a terminal not in use.
- **[#462](https://github.com/gammons/slk/pull/462): skip huddle room work on ordinary pages**: a perf fix to upstream's huddle room code (`huddle_room.go`), which the fork does not have.
- **[#449](https://github.com/gammons/slk/pull/449): Homebrew cask `postflight_steps`, goreleaser check in CI**: upstream's distribution; the fork has its own release config.
- **[#457](https://github.com/gammons/slk/pull/457): Alacritty termimg docs**: upstream docs.
- **[#450](https://github.com/gammons/slk/pull/450): move channelpicker into `internal/bubbles`**: a widget move like #434 (declined). The perf commit `68521f0f` only helps very large channel lists.
- **[#456](https://github.com/gammons/slk/pull/456): fix the channelpicker import in compose tests**: only needed after #450.
