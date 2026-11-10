# Upstream ledger

slk is maintained as a fork of gammons/slk. Upstream changes come in only
when they earn it. This file records every upstream PR that has been
reviewed and what happened to it, so a declined change is not proposed
again. The `slk-upstream` skill reads and updates it.

Reviewed through upstream `13f3bd0f` (2026-11-10).

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
| #341 | fix(slackdesktop): read Secret Service via spec-only D-Bus calls | taken | `17ca18ca`; upstream's vendorHash commit `5f5b373b` not taken; `flake.nix` regenerated from CI before 0.31.0. Linux tests compiled, not yet run |
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
| #383 | fix(ui): let shift+enter and shift+esc submit and cancel search | taken | `eaf9448c` |
| #385 | fix(membership): record a failed member write so the backoff holds | taken | `2b723fc9` |
| #386 | fix(compose): bump the render version in RefreshStyles | taken | `54576415` |
| #381 | fix(ui): clear the link picker's kind when a link is committed | taken | `2727eb8c` |
| #423 | fix(ui): route the activity view through handleGoToTop | taken | `66818bd9` |
| #403 | fix(compose): draw the cursor in the theme's text colour | taken | `7356c736` |
| #390 | fix(messages): collect warm emoji uploads from Block Kit text | taken | `efcb598d`; AGENTS.md row in `3045c2fe` |
| #377 | fix(config): make appearance.show_avatars work | taken | `48534cad`; its test rewritten over `loadConfigString` |
| #394 | test(ui): pin every Mode to a handler with a modeCount sentinel | taken | `fb53527c`; the sentinel sits after `ModeCanvas` |
| #400 | fix(preview): open images without a thumbnail (SVG) in the system viewer | taken | `87056ca2` |
| #409 | fix(messages): keep the theme's text colour on Block Kit lines, attachments and after inline emoji | taken in part | `38aca3fd`. Of `42ed9a0a` only the test (`919fdb9f`): the fork already restores the colour after an image placement with `ReapplyAfterFgResets` |
| #417 | fix(ui): keep avatars intact when author headers wrap | taken | `7d83947d`; long names and avatars added to the `RenderMessage`/`WrapRows` property in `8de189f8` |
| #415 | fix(mentions): resolve user names in message content | taken | `a218634d`; its two timeout selects made blocking receives |
| #316 | fix: copy, export and open links from a forwarded message's shared text | taken | `42e7cd2f`..`b92690ce`. `MessageLinks` stays on the fork's attachment scan from #336 (of `4b3cdb4b` only the test); property for `MessageCopyText` in `e6dbdd81` |
| #375 | fix(messages): show app link chips (attachment_mention) by their title | taken | `b442c3be`, `e9789d08`; title round-trip property in `08b115d7` |
| #402 | fix(messages): keep links to other Slack messages (message_mention) in the body | taken | `3c1af211`; its `richtext_markdown` hunks came in with #395 |
| #442 | fix(mrkdwn): link typed URLs in quotes, headings and after punctuation | ported | `65bfd9a9`, onto the fork's converter: a URL after a quote mark or colon, the tokenizer's `:https:` emoji guard, URLs in nested quotes. Headings and single-level quotes already worked |
| #425 | fix: link more URL schemes, and confirm before opening the unusual ones | taken in part | Typed URLs on localhost and IP hosts, and a closing paren: `1006379e`. `internal/linkscheme`, the file/ssh/ftp/sftp schemes and the confirm prompt declined (Kira, 2026-11-10). `call:tel:…` still sends an emoji element named `tel`, since the fork does not link `tel:` |
| #378 | fix(ui): accept non-ASCII runes in the command line and finders | taken | `50324dbd` |
| #393 | fix(ui): accept non-ASCII runes in the theme, presence, reaction and help searches | taken | `b418bbd2`; the ten input sites share `text.TypedRune` / `text.DropLastRune` in `a79951e8` |
| #398 | feat(thread): p goes to the open thread's message in its channel | taken | `cf902f71`, `096c5326` |
| #440 | fix(ui): close a modal when its mode is left | taken | `f1b2301e`; `closeModal` also closes the canvas viewer |
| #446 | fix the linkpicker import in #440's test | declined | Only needed after #434's move |
| #399 | fix(presence): DM peers shown away while they are online | taken | `8916194a`..`c23e7175`; `2a0b32a4` writes `QueryPresence` over `wsSession` and splits it over frames (a `presence_query` frame is two bytes longer than its `presence_sub`, so one frame could leave the last peer unqueried) |
| #278 | fix(starred): debounce reconnect recovery, include recovered peers in presence, keep newer finder visits | taken | `23874eb2`..`db6fa3d1`; `95b494b2` moves the debounce onto `dedupeGate` with an injected clock. `ec5e7976` left out: its test assumes channels sort first only in the stars section |
| #424 | fix(forward): share the message with chat.shareMessage | taken | `4032d5d0`; `7e5391b2` (no uuid dependency, request checked against the capture); `7f1e68d1` makes a forward a pending insert in `pendingOps` in place of upstream's `forwardStandIns`. Shapes outside the one capture: slk#7 |
| #444 | fix: build permalinks locally where Grid restricts chat.getPermalink | declined | No Enterprise Grid workspace in use (Kira, 2026-11-10) |
| #416 | fix(sidebar): load starred conversations from the boot snapshot | declined | Same reason as #444; `stars.list` works off Grid |
| #389 | feat(emoji): rank frecent emoji first in the reaction picker and autocomplete | ported | `4e333f12`, `9e8ec1b6`. Uses are found through `mrkdwn.Convert` (`mrkdwn.EmojiShortcodes`), recorded when Slack accepts the message, off the Update goroutine. History is still one table for all workspaces: slk#12 |
| #319 | render system messages as centred notices and show who started a huddle | taken in part | Join, leave and canvas-update notices as one muted line: `2779191a`..`5d483039` (`core.IsSystemNoticeSubtype`; kept out of notifications, unread state, `countMentions` and `UnreadStart`; reactions and search matches drawn; reply, edit, delete and forward refused by `noticeGuard`, slk#9). Huddle commits and the centred divider style declined |
| #436 | refactor(core): one list of system-notice subtypes | already in fork | `core.IsSystemNoticeSubtype`, written with the #319 port (`2779191a`) |
| #418 | fix(emoji): render message and status emoji consistently | ported | `85280f03`, `4e00152f`, `437b2477`; the thread parent's emoji uploads, which the fork's thread pane dropped, fixed in the last |
| #395 | fix(compose): edit a message as it was typed (mentions, lists, links) | taken | `6f7a9148`..`5c4c1880`. The seed is rebuilt for the fork's converter and checked against it (`a2cee471`): each shorter spelling is kept only when `mrkdwn.Convert` sends the same block. `63b4783e` fixes a fork bug the properties found (a wire token rewritten when a user shares a group's name). Converter gaps the properties skip: slk#11 |
| #453 | draft preview above the compose box (ctrl+r) | ported | `554df8fb`..`dc94d9f3`. Draws the placeholder row Enter would make, through the pane (`RenderOptions.BodyOnly`, `contentPane.RenderDraft`). `25d4cf0c` fixes a fork bug found on the way: a click on a compose box in insert mode reached the message list. `adcafc2c`'s mrkdwn fix was not needed (pinned by tests in `56dc1098`). The fork's design note is `docs/superpowers/specs/2026-11-10-compose-draft-preview-design.md` |
| #435 | feat(emoji): cache the custom emoji list in SQLite and apply emoji_changed | declined | About 1000 lines and a new table so a new custom emoji shows before a restart (Kira, 2026-11-10) |
| #358 | feat: click links and file lines with the mouse | declined | The `[File]` click came with #347; `o` and the terminal's own click open links (Kira, 2026-11-10) |
| #438 | compact style | declined | Not wanted (Kira, 2026-11-10) |
| #426 | hide deactivated users | declined | About 2900 lines across 47 files; the same shape as #317. `0ec19e3e` (recover open DMs that `users.conversations` leaves out) could be taken alone if DMs go missing |
| #432 | feat(sidebar): Threads and Activity counts as right-aligned badges | declined | A cosmetic option; the fork's Unreads section and status-bar pill cover it |
| #376 | feat(slack): send links to Work Objects as chips, like the web client | declined | A `chat.unfurlLink` per bare link and up to 3s of waiting on every send with a link; #375 already shows received chips |
| #439 | Fix sending/editing multi-paragraph msgs | already in fork | The fork's converter sends `a\n\nb` as one section with its blank lines (probed 2026-11-10, repeated blank lines too) |
| #401 | show a hidden DM once it turns unread | already in fork | The Unreads section's `rebuildFilter` |
| #379 | keep the scroll position inside the parent on resize | already in fork | `entrylist` anchors by entry ID |
| #380 | draw the thread header separator with ─ | already in fork | |
| #405 | "Unread mentions" pills for mentions scrolled out of view | declined | The Unreads section and status-bar pill do this job |
| #407 | optional Slack-like sidebar (indent, look, Active-only filter, private glyph, mouse resize) | declined | Cosmetic options stacked on sidebar options the fork lacks |
| #404 | optional Slack-like names, selected-row pill and mention badges | declined | Cosmetic theme roles, same stack as #407 |
| #406 | optional Slack-style reply row with replier avatars | declined | Opt-in cosmetic, kitty only, changes `core` types and ports |
| #391 | optional author avatars in the thread pane | declined | Opt-in cosmetic |
| #434 | refactor(bubbles): move linkpicker into internal/bubbles | declined | A widget move with no felt payoff |
| #321 | call chrome.Window directly in the dialogs | declined | Refactor churn; conflicts with the grouped help overlay |
| #387 | correct the normal- and insert-mode header comments | declined | The comments describe upstream-only features |
| #326, #324, #323 | benchmarks (whole App with a dialog, chrome, ansi) | declined | Benchmarks only, like #325 |
| #443 | feat(image): sixel and real cell size on Windows Terminal | declined | Windows only |
| #447 | add star history | declined | Upstream's README |
| #367 to #372, `88558eb7` | Renovate pins and action bumps | declined | Upstream's workflows |
