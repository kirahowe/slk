# Upstream ledger

slk is maintained as a fork of gammons/slk. Upstream changes come in only
when they earn it. This file records every upstream PR that has been
reviewed and what happened to it, so a declined change is not proposed
again. The `slk-upstream` skill reads and updates it.

Reviewed through upstream `07bdfab0` (2026-10-19).

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
| #383 | fix(ui): let shift+enter and shift+esc submit and cancel search | taken | `43afe8f8` |
| #385 | fix(membership): record a failed member write so the backoff holds | taken | `46d2c687` |
| #386 | fix(compose): bump the render version in RefreshStyles | taken | `50ecb123` |
| #381 | fix(ui): clear the link picker's kind when a link is committed | taken | `c9f1e298` |
| #393 | fix(ui): accept non-ASCII runes in the theme, presence, reaction and help searches | taken | `c765386b`; the eight inline checks it and #378 leave became `text.IsTypedRune` / `text.DropLastRune` in `0a60dd8b` |
| #378 | fix(ui): accept non-ASCII runes in the command line and finders | taken | `17a8dad1`, onto slk's direct `a.cmdline` writes |
| #390 | fix(messages): collect warm emoji uploads from Block Kit text | taken | `4d65b0fb` |
| #377 | fix(config): make appearance.show_avatars work | taken | `7c484f9e`; test rewritten over `loadConfigString` |
| #394 | test(ui): pin every Mode to a handler with a modeCount sentinel | taken | `ae6330e0`, after `ModeCanvas` |
| #400 | fix(preview): open images without a thumbnail (SVG) in the system viewer | taken | `6fd71c33`; slk's key fixtures gained a thumbnail |
| #417 | fix(ui): keep avatars intact when author headers wrap | taken | `0eaf9477`; the test needing upstream's file hits (#358) cut to reactions; wrap extracted to `wrapAuthorHeader` with a property in `9ae07a2b` |
| #409 | fix(messages): keep the theme's text colour on Block Kit lines, attachments and after inline emoji | taken | `f97fccab`, `318c86e3`; slk's goldens re-blessed in `10d98c80` (text unchanged) |
| #415 | fix(mentions): resolve user names in message content | taken | `718d93b8`; `time.After` waits removed in `6e97b3ed` |
| #336 | fix(ui): o finds links in Block Kit blocks and legacy attachment bodies | taken | `e1266532`, with #316 and #375, which build on it; property in `e3dd0691` |
| #316 | fix: copy, export and open links from a forwarded message's shared text | taken | `9f29c404`..`3c022af7`; property in `de5cf6ac` |
| #375 | fix(messages): show app link chips (attachment_mention) by their title | taken | `e0676d9a`, `b4d41466`; property in `475fbfac` |
| #398 | feat(thread): p goes to the open thread's message in its channel | taken | `141793a5`, `24c8ca99`; grouped-help tag `Threads`; reach-test exemption `52f45101` |
| #399 | fix(presence): DM peers shown away while they are online | taken | `64ebcbf4`, `eaf45eed` (inside `switchLists`' caller), `5f26a707` (rewritten over `wsSession`, only the subscribed prefix, split by `presenceQueryBatches`) |
| #278 | fix(starred): debounce reconnect recovery, include recovered peers in presence, keep newer finder visits | taken | `10965e44`, one ported commit: presence stays in `selfFetch`, debounce on an injected clock, no `time.After`; upstream's unknown-section sidebar order test left out |
| #416 | fix(sidebar): load starred conversations from the boot snapshot | taken | `bddce499`; test over the owner slice |
| #395 | fix(compose): edit a message as it was typed (mentions, lists, links) | taken | `7db8ea03`..`5aa7bec2`: `mentionsForEdit` beside `translateMentions` instead of upstream's `sendMentions`; properties found and fixed names rewritten inside wire tokens (`013a8c8b`), nested lists sent out of order (`d9b11e0d`), and line breaks that assumed upstream's `Convert` (`d7d4204b`) |
| #402 | fix(messages): keep links to other Slack messages (message_mention) in the body | taken | `d915d91e` |
