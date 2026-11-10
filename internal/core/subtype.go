package core

// SubtypeCanvasUpdated is the subtype of the message Slack posts when a
// channel's canvas tab is edited. It often has no text of its own.
const SubtypeCanvasUpdated = "tabbed_canvas_updated"

// IsSystemNoticeSubtype reports whether a message subtype is a system
// notice (someone joining or leaving a channel, private channel or group
// DM, or the channel's canvas being updated) rather than a message
// someone wrote. It is the one list: internal/slack delivers these live,
// messages.RenderMessage draws them as one muted line in both panes, and
// cmd/slk keeps them out of notifications, unread state and mention
// counts.
func IsSystemNoticeSubtype(subtype string) bool {
	switch subtype {
	case "channel_join", "channel_leave", "group_join", "group_leave", SubtypeCanvasUpdated:
		return true
	}
	return false
}
