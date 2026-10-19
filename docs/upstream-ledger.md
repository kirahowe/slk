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
