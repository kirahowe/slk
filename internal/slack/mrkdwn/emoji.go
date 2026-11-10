package mrkdwn

import (
	"regexp"

	"github.com/slack-go/slack"
)

// skinToneName is an emoji element that only sets the skin tone of the
// emoji before it, as ":+1::skin-tone-3:" sends: two elements, "+1"
// then "skin-tone-3". Slack's tones are 2 to 6.
var skinToneName = regexp.MustCompile(`^skin-tone-[2-6]$`)

// EmojiShortcodes returns the emoji input sends: the names of the emoji
// elements in Convert(input)'s rich_text block, in order, without
// colons. A skin-tone element directly after another emoji is joined
// to it in Slack's "<name>::skin-tone-N" form, so ":+1::skin-tone-3:"
// gives "+1::skin-tone-3". A shortcode in a code span or code block,
// and the "https" of "see:https://x", are text, so they are not here.
// Every :name: counts, whether or not it names an emoji.
func EmojiShortcodes(input string) []string {
	_, rt := Convert(input)
	if rt == nil {
		return nil
	}
	var out []string
	var walk func(elements []slack.RichTextElement)
	inline := func(elements []slack.RichTextSectionElement) {
		joinable := false
		for _, e := range elements {
			em, ok := e.(*slack.RichTextSectionEmojiElement)
			if !ok {
				joinable = false
				continue
			}
			if joinable && skinToneName.MatchString(em.Name) {
				out[len(out)-1] += "::" + em.Name
				joinable = false
				continue
			}
			out = append(out, em.Name)
			joinable = !skinToneName.MatchString(em.Name)
		}
	}
	walk = func(elements []slack.RichTextElement) {
		for _, e := range elements {
			switch v := e.(type) {
			case *slack.RichTextSection:
				inline(v.Elements)
			case *slack.RichTextList:
				walk(v.Elements)
			case *slack.RichTextQuote:
				inline(v.Elements)
			case *slack.RichTextPreformatted:
				inline(v.Elements)
			}
		}
	}
	walk(rt.Elements)
	return out
}
