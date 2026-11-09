# Upstream review 2026-11-09

Range: `4428adf0..cd6ca030`. That covers 18 upstream units: 17 merged PRs plus one direct commit (`88558eb7`, Renovate action bumps). #446 is a one-line build fix for #440 and is listed under #440. There are 66 non-merge commits in all. None is already in the fork by patch (`git cherry`), and the ledger lists none of them. The ledger still says "reviewed through `07bdfab0`". Counts: 7 take, 5 maybe, 6 skip.

The most important item is #444. On Enterprise Grid, `Y`/`C` (copy permalink) fails with `enterprise_is_restricted`, and so does the permalink in forward's local echo. The fork has had a Grid user before (#5 in `cmd/slk/main.go`). #444 builds on #424, which makes forward call `chat.shareMessage`, so the recipient gets a real shared message instead of a pasted permalink. Take the two together.

The fork's mrkdwn converter is its own rewrite, so none of upstream's mrkdwn changes apply as patches. Probing the fork's `mrkdwn.Convert` showed three real bugs:
- `see:https://x.y` and `call:tel:+1555` go out with an `emoji` element named `https` or `tel`.
- A URL inside quotes (`"https://x.y"`) is not linked.
- `http://localhost:8080` and IP-host URLs are not linked.

#442 and #425 fix these upstream and need hand-ports onto the fork's `walk.go`/`tokens.go`.

## Take

- **#444: fix: build permalinks locally where Grid restricts chat.getPermalink** ([PR](https://github.com/gammons/slk/pull/444))
  - What: `GetPermalink` asks Slack first. On `enterprise_is_restricted` alone, it builds the link itself with the new `slackurl.Build` (the inverse of `slackurl.Parse`) from auth.test's URL. A reply's link needs its thread, so `GetPermalink`, `ForwardMessage` and the core `Permalink`/`Forward` methods now take the thread ts.
  - Why: copy-permalink is broken on Grid, while Slack's own client works there. This keeps slk working on a workspace type the fork has users on.
  - Conflict: high. It depends on #424's `forward.go`. Applied after #424's first commit, it conflicts in `internal/slack/client.go`, `internal/slackurl/slackurl.go` (and its test), `internal/ui/forward_message_test.go`, AGENTS.md and `wiki/Features.md`.
  - Cherry-pick: conflicts both alone and after #424. Hand-port.
  - Fit:
    - `slackurl.Build` is pure. Add a rapid round-trip property, `Parse(Build(x)) == x`, using `proptest.ID`/`proptest.TS`.
    - Add `slackurl.Build` to the AGENTS.md table next to `slackurl.Parse`.

- **#424: fix(forward): share the message with chat.shareMessage** ([PR](https://github.com/gammons/slk/pull/424))
  - What: forward makes the web client's `chat.shareMessage` call (checked against a scrubbed capture in `testdata/chat_share_message.json`) instead of posting a permalink. The permalink is now fetched only for the local stand-in. Forwarded TSes are remembered (capped), so the share's real echo, which carries the shared attachment, replaces the stand-in.
  - Why: the recipient sees the forward the way Slack shows one, not as a pasted link with an unfurl. It is also the prerequisite for #444.
  - Conflict: high in `internal/ui/reducer_send.go`. Upstream swaps the echo in with `UpsertSelfSent`, exempt from the self-send guard. In the fork, `reduceNewMessage` goes through `pendingOps.HoldEcho`, and rows must go through `messages.Insert`/`Upsert`. AGENTS.md also conflicts.
  - Cherry-pick: `a5a4418a` is clean. `d7b9cb32` conflicts in `reducer_send.go` and AGENTS.md.
  - Fit:
    - Express the echo swap with `messages.Upsert` through the panes' `rowSink`, not a pane-specific upsert.
    - Check the interaction with `HoldEcho`: a forward is not a `pendingInsert`, but another send pending in the destination would hold the echo.
    - Use `assertFormMatchesCapture` for the request test if upstream's test does not already.

- **#442: fix(mrkdwn): link typed URLs in quotes, headings and after punctuation** ([PR](https://github.com/gammons/slk/pull/442))
  - What: a quote or colon before a typed URL is taken as text, so Linkify sees the URL. The tokenizer no longer reads `:https:` or `:tel:` as an emoji when a typed URL starts at its name. Headings link their URLs. Blockquotes become a `rich_text_quote` walked inline, and a nested quote keeps its line breaks.
  - Why: the fork sends `see:https://x.y` as text `see` + emoji `https` + text `//x.y`, so the message arrives without a link. It does the same with `call:tel:…`. `"https://x.y"` is not linked either.
  - Conflict: high. The fork's `internal/slack/mrkdwn/walk.go` is its own rewrite (about 385 lines differ from upstream's base). `convert_test.go` and AGENTS.md also conflict.
  - Cherry-pick: `c35c609b` conflicts in `walk.go`, `convert_test.go` and AGENTS.md. Hand-port.
  - Fit:
    - Port only the gaps the fork has: the punctuation and colon case, and the tokenizer's emoji guard. The fork already links URLs in headings and in single-level quotes, and keeps quote line breaks. Nested quotes (`> > https://…`) still go out as text.
    - The port depends on #425's `typedURLRegexp`, or a fork-local equivalent.
    - Add cases to the fork's `mrkdwn_prop_test.go`, for example "a typed URL from `typedURLRegexp` always comes out as exactly one link element", rather than examples only.

- **#440: fix(ui): close a modal when its mode is left** (with **#446**, its import fix) ([PR](https://github.com/gammons/slk/pull/440), [#446](https://github.com/gammons/slk/pull/446))
  - What: `SetMode` calls a new `closeModal(oldMode)`, which closes whichever modal the old mode owned, the way Esc does. It generalises the existing ModeMarks special case. The link picker and new-message picker clean-up moves into `closeModal`. `TestQuitPromptClosesEveryModal` fails on any modal mode missing from it.
  - Why: ctrl+c from any modal other than marks leaves the modal drawn over the screen and over the quit prompt.
  - Conflict: low. `internal/ui/app.go` conflicts only beside the fork's `exitInsertAfterSend` comment and its inline `cmdline` clear, and `mode_linkpicker.go` conflicts slightly.
  - Cherry-pick: `1a32e378` conflicts in `app.go` and `mode_linkpicker.go`, both mechanical. `b96b1be0` (#446) points the test at `internal/bubbles/linkpicker`. Do not take it unless #434 is taken; the fork still has `internal/ui/linkpicker`.
  - Fit:
    - Add `ModeCanvas` (the fork's canvas viewer) to `closeModal`. Upstream has no such mode, and the test enumerates the modal modes, so check that it covers ModeCanvas.
    - Keep the fork's `cmdline` clear as it is.

- **#418: fix(emoji): render message and status emoji consistently** ([PR](https://github.com/gammons/slk/pull/418))
  - What: expands `:shortcodes:` in Block Kit headers, buttons and accessory labels without reading them as mrkdwn. Draws author-status emoji through the image path. Makes workspace custom emoji shadow built-ins at every alias hop (`emoji.BuildCustomEmojiURL`). `truncateToWidth` becomes `ansi.Truncate`.
  - Why: today app headers and buttons show literal `:shortcode:`. A custom emoji aliased to a name that is also a built-in shows the built-in. A long header can be cut through an escape sequence.
  - Conflict: high. It conflicts in `internal/ui/messages/render_message.go`, `internal/ui/thread/model.go`, two test files, AGENTS.md and `wiki/Features.md`. It touches the same header code as #417 (a take in the last report).
  - Cherry-pick: `9eae8701` conflicts in those six files. Hand-port. The `emoji/url.go` and `blockkit` parts are separable and probably apply alone.
  - Fit:
    - The plain-text path uses `ResolveShortcodesInText`, which keeps the fork's VS16 gate (#301). Confirm that headers and buttons, which are width-sensitive, never reach `emoji.Sprint`.
    - Its test in `internal/ui/messages` uses `select { case <-ready: case <-time.After(2s) }`. Change it to a bare `<-ready`.
    - The alias-shadowing fix in `BuildCustomEmojiURL` is a pure function. Add a property for it.

- **#403: fix(compose): draw the cursor in the theme's text colour** ([PR](https://github.com/gammons/slk/pull/403))
  - What: sets `s.Cursor.Color = styles.TextPrimary` in `compose.New` and `RefreshStyles`.
  - Why: the cursor is drawn in the terminal's default colour, which can be invisible on a light theme. It is a two-line fix.
  - Conflict: low. Only `internal/ui/compose/model_test.go` conflicts, at the end of the file, and three popup goldens change.
  - Cherry-pick: `a9570ada` conflicts only in `model_test.go`. Keep both tests.
  - Fit: re-bless the popup goldens with `go test ./internal/ui -run TestGolden -update`, then check them with `cellDiff`.

- **#423: fix(ui): route the activity view through handleGoToTop** ([PR](https://github.com/gammons/slk/pull/423))
  - What: adds the missing `ViewActivity` arm to `handleGoToTop`, so `gg` works in the Activity feed.
  - Why: `G` works there but `gg` does nothing. Four lines.
  - Conflict: none.
  - Cherry-pick: `a4edcab8` is clean. Build passes and the new test passes.
  - Fit: nothing.

## Maybe

- **#425: fix: link more URL schemes, and confirm before opening the unusual ones** ([PR](https://github.com/gammons/slk/pull/425))
  - What: new `internal/linkscheme` with one allowlist (http(s), ftp(s), sftp, ssh, file, mailto, tel), used by the renderer, `slackfmt`, `ExtractLinks` and the mrkdwn tokenizer. `typedURLRegexp` makes Linkify link URLs with no TLD (localhost, IP hosts, file:). `OpenLinkMsg` to anything other than http(s), mailto or tel opens a confirm prompt first. The prompt shows an escaped, elided target, and `confirmprompt.BodyRoom` sizes it.
  - Judgement: there are two parts. (a) Linking localhost and IP URLs as you type is a plain bug fix; the fork sends them as text today. #442 needs that regexp. (b) Rendering and opening `file:`/`ssh:`/`ftp:` links needs a confirm prompt and about 110 lines of display-safety code. That surface only pays off if such links show up in Kira's workspaces. A reasonable split is to take (a) as part of the #442 port and decline (b).
  - Conflict: high. It conflicts at the first commit in AGENTS.md, `internal/slackfmt/slackfmt.go` and `internal/ui/messages/render.go`. The fork's `walk.go`/`convert.go` differ. Several commits touch `edit_source.go` and `blockkit/richtext_markdown.go`, which exist only if #395 (a take last week) has been ported.
  - Fit: `displayTarget` and `TypesAsBareURL` are pure and have example tests only, so add properties. Their link regexps should come from `linkscheme`, not a fork-local copy.

- **#426: hide deactivated users** ([PR](https://github.com/gammons/slk/pull/426))
  - What: `general.hide_deactivated_users`, on by default. It caches a deactivated flag on users, and hides deactivated users' DMs from the sidebar, finder and pickers. Their DMs stay findable in the channel finder, with the composer blocked. It also recovers open DMs that `users.conversations` leaves out, and has 15 commits of follow-up fixes (unread counts, reactivation races, window title).
  - Judgement: about 2,900 lines across 47 files for an annoyance that matters only in a large, churning workspace. Last week's ledger declined #317 as "large surface for small gain", and this is the same shape. `0ec19e3e` (recover open DMs that users.conversations leaves out) may be worth taking alone, if Kira has seen missing DMs.
  - Conflict: high in `internal/ui/sidebar/model.go` (the fork's Unreads section and `InUnreads`), `internal/ui/app.go`, the channel finder and `cmd/slk`. It conflicts at the fourth commit.
  - Fit: the snapshot handling must follow the owner-slice and map rules in AGENTS.md (`publishChannels`, no shared map writes).

- **#435: feat(emoji): cache the custom emoji list in SQLite and apply emoji_changed** ([PR](https://github.com/gammons/slk/pull/435))
  - What: persists each workspace's custom emoji list in a new cache table and handles the `emoji_changed` socket event. Refetches are coalesced, and an in-flight list is discarded on change or reconnect.
  - Judgement: the fork does not handle `emoji_changed` at all, so a newly added custom emoji shows as `:name:` until restart. That is a real but minor staleness. Against it: about 1,000 lines, three follow-up race fixes in the same PR, and a new SQLite table. Take it if new custom emoji show up often enough to notice.
  - Conflict: high at the first commit, in `internal/slack/events.go`. `cmd/slk/connect.go`, `main.go` and `workspace.go` are touched too.
  - Fit: `cmd/slk/customemoji_test.go` has six `time.Sleep` calls, including a 20ms "room for an unwanted call" quiet period. AGENTS.md forbids all of them. Rewrite them with synctest or done-signal draining. The published emoji map must stay swapped whole via `atomic.Pointer` (`WorkspaceContext.CustomEmoji`).

- **#438: compact style** ([PR](https://github.com/gammons/slk/pull/438))
  - What: `appearance.compact` and a `:compact` command. Together they reduce layout padding, ComposeBox padding (now configurable in `styles`) and message spacing.
  - Judgement: a taste call about density. It adds a config key, a command and a renderer flag to every pane, about 315 lines. Worth it only if Kira wants denser panes. The fork's narrow-terminal layout already handles cramped widths.
  - Conflict: high at the first commit, in `internal/config/config.go`. Layout and goldens conflict with the narrow-terminal work too.
  - Fit: the renderer flag would be a `RenderOptions` field, not a per-pane branch.

- **#432: feat(sidebar): Threads and Activity counts as right-aligned badges** ([PR](https://github.com/gammons/slk/pull/432))
  - What: `[sidebar] right_align_badges` draws the Threads and Activity counts as mention badges at the right end of the row, capped at 99+, as Slack does.
  - Judgement: a cosmetic option, about 60 lines. The fork moved DMs and mentions into an Unreads section with a status-bar pill, so check whether the badges still fit that design.
  - Conflict: high, in `internal/config/config.go`, `internal/ui/sidebar/model.go` and `wiki/Configuration.md`.
  - Fit: none beyond the conflict.

## Skip

- **#439: Fix sending/editing multi-paragraph msgs** ([PR](https://github.com/gammons/slk/pull/439)): fixes upstream's walker. The fork's rewritten converter already sends `a\n\nb` as one section with the blank line kept (probed).
- **#434: refactor(bubbles): move linkpicker into internal/bubbles** ([PR](https://github.com/gammons/slk/pull/434)): a widget move plus windowing for a small modal, with no felt payoff. Taking it would also require #446's import fix.
- **#436: refactor(core): one list of system-notice subtypes** ([PR](https://github.com/gammons/slk/pull/436)): a no-behaviour refactor that mostly names the huddle subtype. Fine in principle, but low value for the fork.
- **#443: feat(image): sixel and real cell size on Windows Terminal** ([PR](https://github.com/gammons/slk/pull/443)): Windows-only console probing. The fork's owner is not on Windows.
- **#447: add star history** ([PR](https://github.com/gammons/slk/pull/447)): an upstream README change.
- **88558eb7: chore(deps): update github actions** ([commit](https://github.com/gammons/slk/commit/88558eb7)): Renovate pin bumps in upstream's workflows.
