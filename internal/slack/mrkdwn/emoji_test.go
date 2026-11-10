package mrkdwn

import (
	"reflect"
	"testing"
)

func TestEmojiShortcodes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"no emoji here", nil},
		{"ship it :rocket: :tada: :rocket:", []string{"rocket", "tada", "rocket"}},
		{"nice :+1::skin-tone-2:", []string{"+1::skin-tone-2"}},
		// A tone after a space is its own emoji.
		{":+1: :skin-tone-2:", []string{"+1", "skin-tone-2"}},
		// Only a plain emoji takes a tone, and only one.
		{":skin-tone-2::skin-tone-3:", []string{"skin-tone-2", "skin-tone-3"}},
		{":+1::skin-tone-2::skin-tone-3:", []string{"+1::skin-tone-2", "skin-tone-3"}},
		// Every :name: is sent as an emoji, known or not.
		{"meet at 12:30:45", []string{"30"}},
		{"`:tada:` and :fire:", []string{"fire"}},
		{"```\n:tada:\n```", nil},
		{"see:https://example.com/x :tada:", []string{"tada"}},
		{"> quoted :wave:", []string{"wave"}},
		{"- item :one:\n  - nested :two:", []string{"one", "two"}},
		{"*bold :tada:*", []string{"tada"}},
	} {
		if got := EmojiShortcodes(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("EmojiShortcodes(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
