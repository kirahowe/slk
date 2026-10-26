# Upstream ledger

slk is maintained as a fork of gammons/slk. Upstream changes come in only
when they earn it. This file records every upstream PR that has been
reviewed and what happened to it, so a declined change is not proposed
again. The `slk-upstream` skill reads and updates it.

Reviewed through upstream `04d0a2d4` (2026-10-26).

| PR | Title | Decision | Notes |
|---|---|---|---|
| #307 | fix(presence): snapshot peers before background bootstrap | taken | `bce75846`, `aa3538ab`; tests fitted in `913cf148`. `RefreshPeerDND` and the workspace switch still read `wctx.Channels` off the event goroutine; upstream has the same reads |
| #322 | perf(scrollbar): build the row pad from a cached style prefix and suffix | taken | `2a9249b9`, `81fc6a15`; conflict with the wide-rune pad (`b630a99b`) resolved onto `padCells`; oracle test replaced by a property in `14817ecb` |
| #317 | disable the composer in app DMs that cannot be messaged | declined | ~1100 lines and an extra request per app DM opened, for a small gain |
| #318 | copy huddle links for conversations and people | declined | Huddles are not used; it also replaces the users.info path |
| #331 | fix(messages): wrap quoted lines under the bar | already in fork | `69e12859`, `9515c481` |
| #320 | perf(bubbles): cache confirmprompt's rendered box | declined | The modal is rarely open; no felt payoff |
| #325 | test(bubbles): add entrylist and pane scaling benchmarks | declined | Benchmarks only; applies cleanly if perf work wants them |
| #330 | chore: add install task to makefile | declined | Trivial; add by hand if wanted |
| #334 | docs: describe the automated reviewer in reviewing-prs.md | declined | Upstream's own review process |
| #342 | fix(image): don't pick kitty for WezTerm under tmux | taken | `331bce9f` |
| #339 | fix(emoji): keep an emoji that follows a skin-toned one | taken | `ad4ec3bf`..`41b0d7b5`; properties and a fuzz target for `StripSkinToneFromText` in `306b89f5` |
| #340 | fix(emoji): draw skin-toned emoji as one image in the right shade | taken | `8bd7510f`, `8c0422d5`; tone-mapping properties in `306b89f5` |
| #351 | fix(emoji): draw two-person emoji with a tone per person | taken | `edbad06a`..`a3d08c01`; tone-per-person property in `306b89f5` |
| #341 | fix(slackdesktop): read Secret Service via spec-only D-Bus calls | taken | `17ca18ca`; upstream's vendorHash commit `5f5b373b` not taken, regenerate `flake.nix` from CI. Linux tests compiled, not yet run |
| #336 | fix(ui): o finds links in Block Kit blocks and legacy attachment bodies | taken | `1c122b95`; property over `proptest.SlackText` in `93b20496` |
| #337 | fix(export): convert inline markup inside quotes to CommonMark | taken | `c253313e`, the conversion line and export tests only |
| #364 | fix: pin the Activity badge to the workspace's unread-activity total | taken | `b290bff6`; UI rollback and zero floor in `4d6f2844`; `badge_clock.go` replaced by `activityBadge` on `selfStatus`'s mark scheme in `6523c14a` |
| #357 | fix: keep the prerender cache across same-protocol reconfigures | taken | `212d1b3e` |
| #363 | fix: keep border and gutter on clipped-image fallback rows | taken | `28d7639e`; graft moved to `bubbles/ansi.Graft` in `e84b18d6` |
| #362 | fix: paint block-kit sixel images through the placement pipeline | taken | `bf3774d8`..`f01fefd1`; `e84b18d6` shares the graft and fixes two cell shifts the property found; comments in `6299093a` |
| #365 | fix: split the prerendered block image's side channel by protocol | taken | `b984cb74` |
| #352 | feat(ui): `:` command Tab completion, candidates, `:help`, `:version` | taken in part | Completion `be62d4a5`, `8d04ce54`, `53050067`, fitted in `47d9e36b`. Upstream's `:help` and `:version` (`08d2b06c`, `e550a067`, `1b2ac531`, `322f2ec8`) not taken; slk's own `:version` and help entries in `6e839459` |
| #348 | fix(messages): apply a message_changed echo fully | taken in part | Cache commits only: `ae168002`, `7664898d`; mention and notify tests ported in `7695fa5a`, `f397cc3a` (the second also stops repeat notifications on edits); `f07bdb14` keeps a subscribed root's edited mark. `c7a51e15`, `a997c0df` already in fork in its own form; `ad50132e` (`entrylist.HoldBottom`) declined, the fork's entrylist anchors a bottom window already |
| #361 | feat: click the thread breadcrumb's channel to go up | taken | `c362e6fb`, `39b6dda7` |
| #347 | fix(ui): clicking a [File] line downloads and opens it | taken | `d0cbca1d`, `086ba5f4`, `ec6caa8c`; `24f53274` fixes fallback lines wrapping and shifting hits below them |
| #359 | feat: click the compose box to start typing | taken | `92450a5b`, `24b34aab` (normal mode only, like `i`) |
| #353 | feat(compose): stay_in_insert_after_send option | declined | Not wanted; its Ctrl handoff clashes with the fork's composer keys |
| #349 | x removes a link preview from your own message | declined | About 2000 lines across 34 files for a rare action |
| #360 | custom status presets (WFH, vacation) in Ctrl+S | declined | Hardcoded presets; truncates with `reflow/truncate`. The own-status segment could be lifted by hand |
| #354 | sort conversations alphabetically within sections | declined | Replaces Slack's order with no option; clashes with the fork's sidebar and goldens |
| #350 | docs: add a Commands wiki page | declined | Upstream's command set differs from the fork's |
| #366 | add stale workflow and README update | declined | Upstream's issue-triage automation |
