## Context

`WorkspaceContext.Channels` (`cmd/slk/workspace.go`) is written by the
workspace's WebSocket event goroutine: `OnConversationOpened` and
`addConversation` append to it (`conversations.go`), and the membership and
section refreshes update items in place (`membership.go`, `sections.go`).
Other goroutines read it with no synchronization:

- `RefreshPeerDND` in the workspace switcher (`cmd/slk/main.go`) calls
  `workspacePresenceIDs(wctx)` inside a `tea.Cmd`.
- The switch function itself reads `withPeerStatuses(wctx.Channels, …)`,
  and runs as a `tea.Cmd` (`App.switchWorkspace`).
- The `Lookup` callback in `wireCallbacks` (`main.go`) ranges over it.
- `railUnreadWorkspaces` (`rail_unread.go`) reads it; its comment admits it.

Upstream #307 (taken in v0.29.1) fixed one reader, the `OnConnect` presence
bootstrap, by snapshotting on the event goroutine. Upstream has the same
remaining reads. A slice header read during `append` is a data race. Under
`-race` the reads are reported, and in production a reader can see a torn
or stale slice.

`FinderItems` is very likely the same shape: `upsertFinderItem` writes it on
the event owner and `Lookup` reads it. Audit it with the same method.

## Design

Fix the class, not the sites. Readers off the owner should be unable to
reach the mutable slice, enforced by the compiler rather than a comment:

- Make the owner's slice private to the event goroutine's code path, and
  publish an immutable snapshot through an `atomic.Pointer` after each
  mutation, the way `customEmoji` / `userGroups` already work (see the
  comments on those fields). Every off-owner reader uses the snapshot
  getter. A published snapshot is never mutated again.
- Code that runs before the socket starts (the connect-time build in
  `connect.go`) may write directly, but must publish before handing the
  workspace to anything concurrent.
- Moving state, not just functions: keep the change behaviour-preserving.
  If you find a behavioural bug on the way, record it in this file instead
  of fixing it in the same commit (AGENTS.md, "Found a bug while
  refactoring?").
- Update AGENTS.md's invariant on shared state ("A map shared across
  goroutines is never written") to cover this slice, and the shared-code
  table if a new accessor is the one to use.

## Acceptance criteria

- [x] No goroutine other than the event owner reads the mutable slice; the
  type system makes it hard to do by accident (unexported field or
  equivalent).
  - Receipt: fields are unexported `channels`/`finderItems`; same package, so
    `TestOwnerSlicesStayOnTheOwnerPath` enforces the owner-file list.
- [x] A `-race` test for each off-owner reader (switch + `RefreshPeerDND`,
  `Lookup`, rail unread) that runs it concurrently with
  `OnConversationOpened`. Each is shown to fail on the old code (record
  the result here). No wall-clock waits; one run, not retry loops.
  - Receipt: `cmd/slk/channels_race_test.go`; four tests failed with DATA
    RACE on the old code (see Log), pass now. Fixed iteration counts, blocks
    on a done channel.
- [x] `FinderItems` audited; fixed the same way if it has the same race, or
  the reason it is safe recorded here.
  - Receipt: same race (upsertFinderItem vs Lookup); fixed with
    `FinderItems()` snapshot. See Log.
- [x] Full gate passes: `go build ./...`, `go vet ./...`, `gofmt -l .` empty,
  `golangci-lint run`, `go test -race -count=1 ./...`.
  - Receipt: all five green, see Log.

## Log

### 2026-10-20 fix

Design. `WorkspaceContext.Channels` and `FinderItems` are now the
unexported `channels` and `finderItems`, owned by the event goroutine (and
`connectWorkspace` before the socket starts). The owner publishes a cloned
snapshot into `atomic.Pointer`s (`publishChannels`, `publishFinderItems`,
`workspace.go`) after each mutation: `addConversation`, `upsertFinderItem`,
`refreshDMPeerFromCache`, `refreshMutedForActive`, `refreshSectionsForActive`,
and once at the end of `connectWorkspace`. Off-owner readers call the
getters `Channels()` and `FinderItems()`. The four readers were extracted
unchanged into named functions so tests can call them: `lookupConversation`
(Lookup callback), `switchLists` (workspace switch; clones the finder
snapshot because the UI owns what it is handed), `workspacePresenceIDs`
(RefreshPeerDND and OnConnect), `railUnreadWorkspaces`/`railRowLights`.
`main.go` also snapshots for the notification name maps and the
WorkspaceReadyMsg clones.

Compiler limits. Everything is `package main`, so an unexported field does
not keep other files out. `TestOwnerSlicesStayOnTheOwnerPath` parses the
non-test files and fails if any file outside a listed owner set names the
fields (and if a listed file stops using them).

Fail-before evidence (`go test -race -count=1 ./cmd/slk -run ConversationOpened`
on the old fields, with the readers extracted): all four failed with
DATA RACE reports (8 in total):
- TestSwitchListsRacesConversationOpened: FAIL
- TestPresenceIDsRaceConversationOpened: FAIL
- TestLookupConversationRacesConversationOpened: FAIL (the report includes
  the `upsertFinderItem` append at conversations.go:79, so FinderItems races)
- TestRailUnreadRacesConversationOpened: FAIL
After the change all four pass.

FinderItems verdict. It had the same race: `upsertFinderItem` appends and
overwrites on the owner, `Lookup` and the switch read it off-owner. The
switch also handed the live slice to the UI inside WorkspaceSwitchedMsg, so
an in-place owner write could land in the UI's copy. Fixed the same way.

Gate: go build, go vet, gofmt -l (empty), golangci-lint (0 issues), and
go test -race -count=1 ./... all pass.

Open: no behavioural bugs found. Test literals that seed `channels`
directly must call `publishChannels()` before exercising a snapshot reader
(done in the existing tests that needed it).
