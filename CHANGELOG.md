# Changelog

All notable changes to this fork are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

Earlier history, up to v0.26.2, is in [upstream's releases](https://github.com/gammons/slk/releases).

## [Unreleased]

### Added

- A draft preview. `Ctrl+R` while typing shows the draft above the
  compose box as the message will look once sent: list bullets, bold,
  links as their labels, mentions as names, and emoji as images where
  the terminal draws them. It follows the draft as you type and keeps
  the end of a long one in view. The channel and thread boxes each have
  their own, and sending or leaving the box closes it.
- The emoji you use come first in the reaction picker and in the `:`
  autocomplete, whatever you have typed. A reaction counts once Slack
  adds it, and the emoji in a message, thread reply or upload caption
  once Slack accepts it.
- `p` with a thread open goes to the thread's message in its channel
  and closes the thread, so a thread opened from Threads or Activity
  leads back to its conversation. `''` returns to the thread.
- Emoji shortcodes in Block Kit headers, button labels and accessory
  labels are drawn as emoji: `:white_check_mark: Resolve` shows the
  check mark.
- In image-emoji mode an author's status emoji beside their name is the
  same image as in the message body.

### Changed

- `F` forwards a message the way Slack's own Forward does. The
  destination gets a shared message with the original embedded in it,
  where it used to get a link to the original, and the forward shows
  there at once. Enter on a forwarded message opens its thread, as on
  any message; `o` offers the original.
- `E` opens a message as it was typed: `- ` lists, `**bold**`,
  `[label](url)` links and line breaks, with people, channels and
  groups by name. It used to show Slack's wire text, such as
  `<#CUJL4QPU0>` for a channel. Saving an edit you did not change sends
  the same message.
- Join, leave and canvas-update messages are drawn as one muted line.
  They do not notify, mark a channel unread or count as a mention, and
  a canvas update arrives live where it used to show up later as an
  empty Slackbot message. Reactions on them are drawn and can be
  clicked. Replying to, editing, deleting or forwarding one is refused.

### Fixed

- Presence dots on DMs are right after connecting and after switching
  workspaces. Someone already online when slk connected kept whatever
  the cache held until they next went away and came back, a workspace
  switch showed everyone as they were at connect time, and a reply from
  someone could reset them to away. The channel finder's dots follow
  presence as the sidebar's do; they used to stay as they were when slk
  connected.
- A mention inside a message shows the person's name, in forwarded
  messages too, and fills in when the name arrives.
- `y` copies the shared text of a forwarded message, the markdown
  export includes it, and `o` offers the links in it.
- A link that an app turns into a chip (a Linear issue, say) shows by
  its title. It used to vanish from a message that also had text.
  `&amp;` in a link's label shows as `&`.
- A typed URL on `localhost` or an IP address is sent as a link, and so
  is one right after a quote mark or colon (`see:https://x`) or inside
  a nested quote. A closing paren or square bracket after a URL stays
  out of the link unless the URL opened it.
- Text of a list item that comes after a nested list is sent. It used
  to be dropped from the message.
- A line that is only `>` is sent as typed. It used to disappear and
  pull the lines either side of it onto one line.
- A quoted line that is only `-` or `>` is sent. It used to be left out
  of the message.
- A link whose label is HTML-shaped text (`[<a>](https://example.com)`)
  or holds an address in angle brackets is sent with that label. It
  used to go out with part of the label, or none, missing.
- A long author name wraps beside the avatar in a narrow pane and the
  avatar stays in one piece. The timestamp stays whole.
- Plain lines in Block Kit messages and attachments keep the theme's
  text colour. They used to take the terminal's default foreground.
- Custom emoji in Block Kit text and attachments show their image, and
  so do emoji that only a thread's parent message uses. Their cells
  used to stay blank.
- A workspace emoji wins over a built-in of the same name when it is
  reached through an alias.
- Mentioning a user group whose handle is also someone's display name
  sends the group mention intact.
- An email address is no longer sent as a mention when the part after
  the `@` is someone's name (`alice@bob.com` with a user named bob),
  and `page#general` no longer names a channel. A `@` or `#` right
  after an ASCII letter or digit is left as typed.
- A mention of someone who no longer exists costs one request to Slack
  per session. It used to cost one every time the channel was read.
- Sending or previewing a draft with thousands of `:` or `<` in a row
  no longer freezes slk.
- `O` / `v` on an image Slack made no thumbnail for (an SVG) downloads
  it and opens it in the system viewer. Nothing used to happen.
- The `:` prompt and every dialog's search box take non-ASCII
  characters, and backspace removes a whole character. Every search
  box takes a space too, so a DM can be found by first and last name.
- Shift+Enter and Shift+Esc submit and cancel a search.
- `gg` goes to the top of the Activity view.
- Ctrl+C's quit prompt is no longer hidden behind an open dialog, and
  cancelling it no longer leaves a dialog on screen over live keys.
- The compose cursor is drawn in the theme's text colour. It vanished
  on light themes.
- A click in another pane while you are typing leaves the keyboard
  with the box you are typing in. The next character used to be
  dropped, and Enter could send the other box's draft. While typing, a
  click on the sidebar, a thread card or an Activity item does nothing,
  and a click on a message selects it without opening its thread.
- A click on a compose box while typing, with the list scrolled up, no
  longer selects a message and opens its thread.
- `appearance.show_avatars = false` hides avatars. The setting was read
  from the config and never used.

## [0.31.0] - 2026-11-10

### Added

- Clicking works in more places. A click on a `[File]` line downloads the
  file and opens it. A click on the channel name in a thread's breadcrumb
  closes the thread. A click on a compose box starts typing in it, the way
  `i` does (from normal mode only, so a half-written edit or `:` command
  is never lost).
- Tab and Shift+Tab complete `:` command names, and the status bar lists
  the commands that match what you have typed.
- `:version` shows the build you are running, and the update hint when a
  newer release is out. `:help` now lists every `:` command.

### Fixed

- Text after an inline image emoji keeps its colour. On a light theme it
  used to turn grey until the end of the line.
- Skin-toned emoji draw as one image in the right shade, not as the yellow
  emoji followed by a swatch, and the tones were one shade off. Two-person
  emoji with a different tone for each person (a two-tone handshake) draw
  with both tones. An emoji right after a skin-toned one is no longer
  dropped.
- The sidebar's Activity badge shows the workspace's real unread Activity
  count. It used to count only the unread items in the loaded page of the
  feed, so it was often too low, and it changed when you toggled the
  unread filter.
- An edit or link preview arriving for a message no longer resets a
  thread's reply count, drops a broadcast reply from the channel, or keeps
  the channel from refetching messages missed during a reconnect. Editing
  a message that mentions you no longer sends another desktop
  notification. An edited thread root keeps its `(edited)` label.
- `o` finds links inside Block Kit blocks and legacy attachments, such as
  Jira's link previews, instead of saying the message has none.
- Images in Block Kit messages and legacy attachments draw in the right
  place with sixel, and show in the thread pane. A partly scrolled-off
  image keeps the message border and gutter. Switching workspaces no
  longer re-encodes every visible image.
- A long file name in an attachment no longer shifts the click targets
  of the images, files and reactions below it.
- Markdown export converts links, mentions and formatting inside quotes.
- `--add-workspace` reads the Slack desktop session from Secret Service
  providers other than gnome-keyring.
- WezTerm under tmux falls back to halfblock images instead of choosing
  kitty images, which it cannot draw there.

## [0.30.0] - 2026-10-22

### Added

- A read-only canvas viewer. `c` (or `:canvas`) opens the focused
  conversation's canvas full screen: a channel's canvas, or the notes canvas
  on a DM. A canvas shared in a message now shows as `[Canvas]` with its
  title, and `c` on that message opens it. In the viewer, `tab` moves
  between canvases, `o` opens the canvas in Slack for editing and `r`
  fetches it again. Headings, lists, quotes, code, links and mentions are
  drawn; checklists, tables and images are not yet.
- An Unreads section at the top of the sidebar, like Slack's "show unreads
  in a separate section" option. DMs and group DMs with unread messages, and
  any conversation that mentions you (muted ones included), move there until
  read. Channels that are unread without mentioning you stay where they are.
  The conversation you have open stays in the section until you leave it.
  Turn it off with `[sidebar] unreads_section = false`.
- A status bar pill, `Unreads N`, counting those conversations, so a waiting
  DM shows even when the sidebar is scrolled, collapsed or hidden.

### Changed

- `a` and `A` (next and previous unread) go to the Unreads section first when
  you are outside it, and now also stop at a muted channel that mentions you.

### Fixed

- Workspaces stay connected. A workspace with no activity for a minute
  used to drop its connection and reconnect, and one with more than about
  290 DM partners was disconnected five seconds after every connect, so it
  never stayed connected for long. Either way, messages arrived in bursts
  and your own messages could seem slow to send. Reconnects now also back
  off properly when Slack refuses them, and a connection that stops
  responding is dropped and reopened instead of hanging. On a workspace
  with that many DM partners, live presence covers the roughly 280 you
  starred or visited most recently, because Slack's connection takes no
  more.
- slk no longer breaks its own config on a fresh install. The first run
  added a second `[workspaces.*]` block for the workspace it had just set
  up, and every launch after that failed with `workspaces "<slug>" and
  "<team id>" both reference team_id` until you deleted the extra block by
  hand. Found and fixed by [Avi Flax](https://github.com/aviflax) in
  [#1](https://github.com/kirahowe/slk/pull/1). Saving a theme, sidebar
  width or version stamp now also refuses to write a config slk would not
  load, and handles a blank line inside a workspace block.

## [0.29.2] - 2026-10-20

### Fixed

- Fixed data races on each workspace's sidebar and finder lists. Switching
  workspaces, looking up a conversation's name and drawing the workspace
  rail's unread dots read them while new conversations were being added. A
  workspace switch could also hand the finder a list that later changed
  under it.

## [0.29.1] - 2026-10-20

### Fixed

- In the Threads and Activity views, message keys no longer act on a message
  you can't see. The list sits where the channel pane goes, but the pane
  behind it kept its last channel and selection, so `L` listed that
  message's reactions and keys such as `r`, `E`, `D`, `y` and `o` acted on
  it. In Activity, Enter on a row could open a forwarded link from it.
- On the selected card in the Threads list, the selection tint now runs the
  full width of the line. Text after the channel name used to sit on the
  terminal's background.
- Fixed a data race on connect and reconnect: presence setup read the list
  of conversations while newly opened DMs were being added to it. From
  upstream #307.

### Changed

- The scrollbar draws about twice as fast in long panes. From upstream #322.

## [0.29.0] - 2026-10-15

### Changed

- Each sidebar section lists its channels first, then people, apps and group
  DMs, each group in alphabetical order. Until now only the Starred section
  did this, so apps and DMs in other sections were mixed in among the
  channels.
- Edits and deletes show at once. An edit appears in every pane showing the
  message as soon as you press Enter, and a deleted message leaves right
  away. If Slack refuses, the message goes back to what it was: a failed
  edit reopens the editor with your text, and a failed delete puts the
  message back where it was.
- A message that arrives while you have moved up the channel or thread no
  longer pulls the cursor and the view down to it. "-- more below --" shows
  instead, the view stays where you were, and the message stays unread until
  you scroll down to it.
- A failed send, reply or edit always gives your text back. It goes back to
  the composer, or, when you have already typed something new there, onto
  the clipboard together with any other unsent text, and the toast says so.

### Fixed

- The update hint now shows up within a few hours of a release. slk asked
  GitHub at most once a day and only at startup, so a release published
  after that check went unnoticed until the next day, or until a restart.
- Your own messages no longer go missing or show twice. For a few seconds
  after each send slk hid every message of yours arriving in that channel,
  which could hide your file uploads and messages sent from another device
  until a reload, and a sent message could sometimes appear twice.
- Messages in a pane stay in time order, even when they arrive out of order.
- A request to Slack that hangs now times out. Sends, edits, deletes,
  reactions and history fetches could wait forever, and a stuck send kept
  your own new messages in that conversation from showing.
- Keys pressed right after Enter act on the message you just sent. Pressing
  D or e quickly could act on the message before it.
- Edits show their new text. An edited message kept drawing its old text
  until a reload, and "(edited)" disappeared after any reload. Link previews
  and reply-count updates no longer mark a message edited.
- A pending send, edit or delete survives a reload of the channel or thread.
- Slack's read position never moves backward on its own. A message that
  showed up late could mark newer messages unread again.
- A reply is counted once in its parent's reply count, and a parent loaded
  from the cache keeps its count when an event for it arrives.
- The view stays put when a message above it changes height, and a selected
  message is never pushed out of view. At the bottom of a pane, the last
  message is no longer half hidden under "-- more below --".
- The "── new ──" line no longer appears above your own message.
- The mention, channel and emoji pickers let Enter through once nothing
  matches what you typed. The first Enter used to be swallowed.
- A link whose label is empty is drawn as a plain link instead of the URL
  followed by a stray "|".
- A failed channel join or older-history load shows a toast instead of
  failing silently.

## [0.28.0] - 2026-10-15

### Added

- Messages that mention you stand out, the way they do in the Slack desktop
  app. A message that names you gets a yellow bar in the left gutter, a faint
  warm tint across the row, and a pill behind your `@name`. An `@here`,
  `@channel` or `@everyone` message gets only the bar. Your own messages are
  never marked. Custom themes can set `mention_bar`, `mention_bg` and
  `mention_pill_bg`; they default to shades of the theme's `warning` colour.

### Changed

- The help overlay (`?`) credits this fork, made by Kira Howe, with a fainter
  line under it crediting Grant Ammons, who wrote slk up to v0.26.
- Channels in each sidebar section are listed alphabetically, ignoring case and
  accents. Channels pinned with a `:N` suffix in a config section still come
  first, and the Starred section still lists channels before people. slk can't
  read the sort order chosen in the official Slack client.
- When a channel matches more than one config section with the same `order`,
  it goes in the one whose name sorts first. It used to land in a different
  one from run to run.

### Fixed

- Sending a message that starts with a number and a full stop no longer
  renumbers it: "2024. was a year" was sent as "1. was a year", and a list
  typed from 5 was sent counting from 1.
- Text typed on the same line as an opening ```` ``` ```` is no longer dropped
  when the message is sent.
- A block quote that wraps onto more than one line keeps its bar and muted
  colour on every line, not just the first.
- Bold, italics, links and mentions inside a block quote are formatted
  instead of showing their raw markup.
- A line typed right after a list, without indenting it, is sent as its own
  line instead of being joined onto the last list item.
- Loading older history into an empty channel could crash slk.
- Older history could show a message twice after one of your own messages was
  sent.
- When a message or thread reply failed to send, the cursor jumped to the next
  message. It now stays where it was.
- Removing a reaction in a channel could show the wrong reactions on the open
  thread's parent message.
- A config section named like a built-in one (for example `Channels`) was drawn
  twice in the sidebar.
- Page Down in the sidebar could move the cursor up after a fast scroll.
- The channel finder ranked names with accents, CJK or emoji below equally close
  plain-ASCII names.
- Activity and Threads cards with a link in them could run one column past the
  edge of the pane.
- Copying a selection that starts with a combining accent no longer drops the
  accent.
- A wide character (CJK or emoji) where the scrollbar is drawn no longer shifts
  the scrollbar a column to the left on that row.
- A blank entry in `on_keyword` sent a desktop notification for every message.
  Blank entries are now ignored.
- `slk export` in a time zone whose daylight saving time once began at midnight
  (such as America/Sao_Paulo) started those days an hour early.
- Saving a theme whose name contains the DEL character wrote a config file slk
  could not load.
- On Linux, `slk open` with a request slk rejects for length could report
  "connection reset by peer" instead of slk's reason.
- Installing or upgrading the Homebrew cask no longer prints a warning that
  `postflight` is deprecated.

## [0.27.0] - 2026-10-15

First release of the kirahowe/slk fork. It starts from upstream's main after
v0.26.2.

### Added

- `slk open <permalink>` shows a Slack message link in the slk that is already
  running, the way `o` does, so a script or URL handler can try slk before the
  browser. It exits `0` when slk showed the link, `1` when slk declined it, `2`
  on a usage error and `3` when slk is not running. slk declines with `slk is
  busy: <what>` while a dialog, a `:` command, a search, the image preview, a
  message edit or an upload is in the way. If you are composing, it leaves
  insert mode and keeps your draft. The socket is `run/slk.sock` in the data
  directory (`~/.local/share/slk/run/slk.sock`, or under `$XDG_DATA_HOME`).
  Not available on Windows.
- slk tells you when a newer release exists. Once a day it checks GitHub for
  the latest stable release of kirahowe/slk, shows a 12-second status-bar toast
  with the upgrade command for your install, and then leaves a muted
  `↑ <version>` on the right of the status bar. `slk --version` prints the same
  hint. Set `check_for_updates = false` under `[general]` to turn it off. Dev
  builds skip the check.
- `:help` opens the keybinding overlay, the same as `?`.
- `:quit` and `:qa` quit with the same confirmation as `Q`.

### Changed

- **`Ctrl+X` now opens the draft in `$EDITOR`.** `Ctrl+E` used to do this. It
  now moves to the end of the line, and `Ctrl+A` to the start, in both the
  channel and thread composers.
- **`:q` on the last window now quits**, with the `:quit` confirmation, like
  vim. With more than one window it still closes the focused one. `Ctrl+w q`
  keeps toasting "Cannot close last window".
- **The sidebar hides itself on narrow terminals.** It stays off screen while
  the channel pane beside it would be under 60 columns, which is below 100
  columns with the default widths. `Ctrl+b` still toggles it at any width.
  Showing a hidden sidebar pins it until you hide it again.
- Threads open beside the channel from 120 columns, down from 162. The sidebar
  gives way first: 120 to 151 columns show the channel and thread, and 152 and
  up show all three. Below 120 they stack as before.
- **Emoji written as a base character plus VS16, such as the heart, now draw as
  the bare glyph** instead of falling back to `:shortcode:` text, in messages,
  reaction pills, the reaction picker and viewer, sidebar section headers and
  peer status. Some terminals may draw the glyph in text style rather than
  colour. ZWJ sequences, flags, skin tones and keycaps still show the
  shortcode.
- Releases, the Homebrew cask (`brew install --cask kirahowe/tap/slk`) and the
  update check now use kirahowe/slk. `go install` still installs upstream,
  because the Go module path is unchanged, so the update hint no longer
  suggests it.
- The `?` overlay groups its entries under Navigation, Search, Messages,
  Threads, Composer, Windows, Views, Commands and App. It lists `Ctrl+U`
  (clear message input), drops the "Press / to search" line, and shows the
  filter line only while you are searching.
- The pane scrollbar track is a dotted `┊` line, so it no longer looks like a
  second border next to the pane's own.
- The thread breadcrumb is inset one column to line up with the channel header,
  and the rule under it uses box-drawing characters.
- The presence menu's filter placeholder reads `Type to filter...` like the
  other filter boxes.

### Fixed

- Line breaks, blank lines and repeated spaces typed in the composer are now
  preserved in sent messages.
- Sidebar section emoji such as `:hammer_and_wrench:` and `:mountain:` no longer
  push the sidebar edge, message pane and overlay borders one cell off (#301).
- The reaction picker no longer shows a literal `:heart: heart` row among the
  glyphs.
- The sidebar no longer shows two cursor bars while Activity is open. The
  last-viewed channel drops its `▌` marker while Threads or Activity is the
  view on screen.

[0.31.0]: https://github.com/kirahowe/slk/releases/tag/v0.31.0
[0.30.0]: https://github.com/kirahowe/slk/releases/tag/v0.30.0
[0.29.2]: https://github.com/kirahowe/slk/releases/tag/v0.29.2
[0.29.1]: https://github.com/kirahowe/slk/releases/tag/v0.29.1
[0.29.0]: https://github.com/kirahowe/slk/releases/tag/v0.29.0
[0.28.0]: https://github.com/kirahowe/slk/releases/tag/v0.28.0
[0.27.0]: https://github.com/kirahowe/slk/releases/tag/v0.27.0
