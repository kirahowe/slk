# Keep the emoji history per workspace

## Context

The `frecent_emoji` cache table is one history for every workspace.
Since the frecency port (upstream #389, 2026-11-10) it also ranks the
`:` autocomplete, and custom emoji are recorded in it. A picker only
offers emoji the current workspace has, so another workspace's custom
emoji never show, but they still take some of the 30 ranked slots.

Making it per workspace needs a schema change (a `workspace_id`
column and a migration), which is why the port left it alone.

## Acceptance criteria

- [ ] Decide whether one shared history is what is wanted. If not, the table is keyed by workspace and the loaded ranking follows `a.activeTeamID` on a switch.

## Plan

## Log
