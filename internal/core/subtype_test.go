package core

import "testing"

func TestIsSystemNoticeSubtype(t *testing.T) {
	cases := map[string]bool{
		"channel_join":       true,
		"channel_leave":      true,
		"group_join":         true,
		"group_leave":        true,
		SubtypeCanvasUpdated: true,
		"":                   false,
		"bot_message":        false,
		"thread_broadcast":   false,
		"huddle_thread":      false,
		"channel_topic":      false,
	}
	for subtype, want := range cases {
		if got := IsSystemNoticeSubtype(subtype); got != want {
			t.Errorf("IsSystemNoticeSubtype(%q) = %v, want %v", subtype, got, want)
		}
	}
}
