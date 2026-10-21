# Sidebar Unreads section and status bar pill

## Problem

Unread DMs are easy to miss. The sidebar marks them (bold row, mention
badge), but DMs sort alphabetically in a section that comes after every
custom or Slack-native section. In a workspace with a hundred channels
that section is usually scrolled out of view, so the marking is never
seen. The user notices a DM only when a phone notification arrives.

## Design

Two pieces, modelled on Slack's "Show unreads in a separate section"
option.

### 1. An Unreads section at the top of the sidebar

A section titled "Unreads" sits directly below the Threads and Activity
rows. It holds every conversation that needs the user:

- any row with a mention badge (`ChannelItem.MentionBadge > 0`). This
  covers @-mentions and broadcasts in channels, muted ones included,
  since mentions pierce mute, and app DMs, whose every message counts;
- any unmuted DM or group DM with unread messages
  (`IsVisiblyUnread`), so a DM shows up even if its mention count is
  zero.

A channel that is merely unread stays in its home section. The user
has around a hundred of those, and promoting them would bury the DMs
again.

Rows move rather than mirror: a conversation in Unreads does not also
appear in its home section. When it is read it goes back.

The conversation open in the message pane stays in Unreads until the
user leaves it, as in Slack. Opening a DM marks it read, and without
this the row would jump out from under the user the moment they open
it. Only a conversation that was already in the section stays there;
a read conversation never enters it by being opened.

Order within the section is DMs, group DMs and apps first, then
channels; ties by name, as in every other section. Slack orders by
recency, but the sidebar holds no latest-message timestamp to sort by.

The section exists only while it has a row. It collapses and expands
like any other section, and its collapsed header shows the usual
aggregates. Its key is a fixed internal string that cannot be a Slack
section ID or a config section name, so it cannot collide with a
user's own section called "Unreads".

Membership is a function of read state, which changes without the item
list changing. `Model.Invalidate`, which the App calls on every
read-state change, therefore recomputes membership too and, when it
changed, re-sorts and rebuilds the nav with the cursor kept on the same
conversation (`currentCursorKey` / `rebuildNavWithCursor`).
`SetReadStateReader` does the same.

### 2. A status bar pill, and `a` going to Unreads first

The status bar shows ` Unreads N ` on the right, N being the number of
conversations in the section, not counting the open one held there.
The pill reuses the existing `SetUnreadCount` slot, which nothing sets
today. It is updated wherever the sidebar's read state or items change
(`notifyReadStateChanged`, `SetChannels`). It answers "is anyone waiting
for me?" whatever the sidebar's scroll position, focus or collapse
state.

`a` / `A` (next / previous unread) already walk the sidebar in display
order, which now starts with Unreads. From a conversation outside the
section, they go into it first: `a` to its first row, `A` to its last.
Inside it, or once it is empty, they walk as before. The walk also
stops on muted rows with a mention, which `IsVisiblyUnread` used to
skip. No new key is needed.

## Not doing

- Sorting the DM section by recency. Unread DMs already move to the
  top, so the sort order would only affect conversations already read.
- Mention counts on the workspace rail for other workspaces.
- A pill that counts other workspaces.

## Testing

- A property test over random items and read states: every row is in
  Unreads exactly when the predicate holds (or it is the open one and
  was already there); no row appears twice; the nav visits Unreads
  before any other section.
- Examples: a DM moves up when it becomes unread and back when read;
  the open conversation stays put after it is read and leaves when
  another opens; the cursor stays on its conversation across a move;
  an unread channel with no mention stays where it is; a muted channel
  with a mention moves.
- `a` from outside Unreads lands on its first row; the status bar pill
  shows and hides with the count.
