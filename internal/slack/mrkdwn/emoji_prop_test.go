package mrkdwn

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// EmojiShortcodes finds exactly the emoji the text was built from: a
// message made of plain words, :name: shortcodes, toned shortcodes,
// clock times, shortcodes in code spans and typed URLs that follow a
// colon gives the names of the shortcodes in order, a tone joined to
// its emoji, the middle of each clock time (Convert sends ":30:" of
// "12:30:45" as an emoji), and nothing for a code span or a URL.
func TestEmojiShortcodesFindsTheShortcodesSent(t *testing.T) {
	word := rapid.StringMatching(`[a-zA-Z0-9.,éñ漢😀]{1,6}`)
	name := rapid.StringMatching(`[a-z0-9_+]{1,8}`)
	rapid.Check(t, func(t *rapid.T) {
		var in strings.Builder
		var want []string
		for i := range rapid.IntRange(0, 8).Draw(t, "segments") {
			if i > 0 {
				in.WriteString(rapid.SampledFrom([]string{" ", "\n"}).Draw(t, "sep"))
			}
			switch rapid.IntRange(0, 5).Draw(t, "kind") {
			case 0:
				in.WriteString(word.Draw(t, "word"))
			case 1:
				n := name.Draw(t, "emoji")
				in.WriteString(":" + n + ":")
				want = append(want, n)
			case 2:
				n := name.Draw(t, "toned")
				tone := "skin-tone-" + strconv.Itoa(rapid.IntRange(2, 6).Draw(t, "tone"))
				in.WriteString(":" + n + "::" + tone + ":")
				want = append(want, n+"::"+tone)
			case 3:
				mm := strconv.Itoa(rapid.IntRange(10, 59).Draw(t, "minutes"))
				in.WriteString("12:" + mm + ":45")
				want = append(want, mm)
			case 4:
				in.WriteString("`:" + name.Draw(t, "code") + ":`")
			case 5:
				in.WriteString(word.Draw(t, "before") + ":https://example.com/" + name.Draw(t, "path"))
			}
		}
		got := EmojiShortcodes(in.String())
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EmojiShortcodes(%q) = %q, want %q", in.String(), got, want)
		}
	})
}

// emojiElementRe finds an emoji element in the JSON of a rich_text
// block, as Slack receives it.
var emojiElementRe = regexp.MustCompile(`"type":"emoji","name":("(?:[^"\\\\]|\\\\.)*")`)

// emojiElementsInJSON is the names of the emoji elements in the JSON
// Convert(in)'s block is sent as, in order: read from the wire form, so
// it does not walk the block the way EmojiShortcodes does.
func emojiElementsInJSON(t *rapid.T, in string) []string {
	_, rt := Convert(in)
	if rt == nil {
		return nil
	}
	b, err := json.Marshal(rt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out []string
	for _, m := range emojiElementRe.FindAllSubmatch(b, -1) {
		var name string
		if err := json.Unmarshal(m[1], &name); err != nil {
			t.Fatalf("unquote %s: %v", m[1], err)
		}
		out = append(out, name)
	}
	return out
}

// Every name EmojiShortcodes returns is one Convert sends as an emoji
// element, for any text made of the pieces that matter here (colons,
// shortcodes, tones, code ticks, emphasis, quotes, lists, URLs): the
// names, with each joined tone split off again, are the emoji elements
// of the JSON Slack receives, in order.
func TestEmojiShortcodesAreConvertsEmojiElements(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pieces := rapid.SliceOf(rapid.SampledFrom([]string{
			":tada:", ":+1:", ":skin-tone-3:", ":skin-tone-7:", ":", "tada", "`", "```", "*", "_", "~",
			"> ", "\n", "\n\n", "- ", "1. ", " ", "word", "12:30", "see:https://x.io/a",
		})).Draw(t, "pieces")
		in := strings.Join(pieces, "")
		var split []string
		for _, sc := range EmojiShortcodes(in) {
			split = append(split, strings.Split(sc, "::")...)
		}
		elements := emojiElementsInJSON(t, in)
		if !reflect.DeepEqual(split, elements) {
			t.Fatalf("EmojiShortcodes(%q) split = %q, Convert's emoji elements = %q", in, split, elements)
		}
	})
}
