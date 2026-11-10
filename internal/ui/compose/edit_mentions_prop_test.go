package compose

import (
	"slices"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/slack/mrkdwn"
	"pgregory.net/rapid"
)

// mentionShape splits s into the mentions Convert reads in it (kind and
// ID, labels dropped) and the text around them, so two sends compare
// equal when they mention the same people, channels and groups in the
// same places.
func mentionShape(s string) (ids []string, rest string) {
	var b strings.Builder
	last := 0
	for _, m := range mrkdwn.Mentions(s) {
		b.WriteString(s[last:m.Start])
		b.WriteString("\x00")
		ids = append(ids, string(m.Kind)+m.ID)
		last = m.End
	}
	b.WriteString(s[last:])
	return ids, b.String()
}

// editText draws Slack text whose mentions use the maps' IDs about half
// the time.
func editText(t *rapid.T, names, channels, groups map[string]string) string {
	var users, chans []string
	for _, id := range names {
		users = append(users, id)
	}
	for _, id := range channels {
		chans = append(chans, id)
	}
	slices.Sort(users)
	slices.Sort(chans)
	text := proptest.SlackText(users, chans).Draw(t, "text")
	if len(groups) > 0 {
		var ids []string
		for id := range groups {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		id := rapid.SampledFrom(ids).Draw(t, "group")
		text += rapid.SampledFrom([]string{" <!subteam^" + id + ">", " <!subteam^" + id + "|@" + groups[id] + ">", ""}).Draw(t, "groupToken")
	}
	return text
}

// Saving an edit unchanged sends what the message already said: for
// any Slack text x, translateMentions(mentionsForEdit(x)) mentions the
// same IDs in the same places, around the same text, as
// translateMentions(x), whose tokens translateMentions leaves alone.
func TestMentionsForEditSendsTheSameMentions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names, channels, groups := genMaps(t)
		x := editText(t, names, channels, groups)
		edited := mentionsForEdit(x, names, channels, groups)
		gotIDs, gotRest := mentionShape(translateMentions(edited, names, channels, groups))
		wantIDs, wantRest := mentionShape(translateMentions(x, names, channels, groups))
		if !slices.Equal(gotIDs, wantIDs) || gotRest != wantRest {
			t.Fatalf("edit of %q is %q, which sends %q, want the mentions of %q",
				x, edited, translateMentions(edited, names, channels, groups), translateMentions(x, names, channels, groups))
		}
	})
}

// Every mention the maps can name is shown by name: a run of tokens for
// known IDs, separated by spaces, comes back with no wire token left,
// as long as no two kinds of mention share a name (a user's name wins
// over a usergroup's handle, which then has to stay a token).
func TestMentionsForEditNamesWhatItKnows(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names, channels, groups := genMaps(t)
		taken := map[string]bool{"here": true, "channel": true, "everyone": true}
		for name := range names {
			if taken[name] {
				t.Skip("a user is named like a broadcast")
			}
			taken[name] = true
		}
		for _, handle := range groups {
			if taken[handle] {
				t.Skip("a handle is taken")
			}
			taken[handle] = true
		}
		var tokens []string
		for _, id := range names {
			tokens = append(tokens, "<@"+id+">")
		}
		for name, id := range channels {
			tokens = append(tokens, rapid.SampledFrom([]string{"<#" + id + ">", "<#" + id + "|" + name + ">"}).Draw(t, "channel"))
		}
		for id, handle := range groups {
			tokens = append(tokens, rapid.SampledFrom([]string{"<!subteam^" + id + ">", "<!subteam^" + id + "|@" + handle + ">"}).Draw(t, "group"))
		}
		tokens = append(tokens, "<!here>", "<!everyone|@everyone>")
		slices.Sort(tokens)
		order := rapid.Permutation(tokens).Draw(t, "order")
		x := strings.Join(order, " ")
		got := mentionsForEdit(x, names, channels, groups)
		if wireTokenRe.MatchString(got) {
			t.Fatalf("mentionsForEdit(%q) = %q, still has a token", x, got)
		}
	})
}

// Text without a mention token is returned as it is.
func TestMentionsForEditLeavesPlainText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names, channels, groups := genMaps(t)
		x := proptest.PlainText().Draw(t, "text")
		if got := mentionsForEdit(x, names, channels, groups); got != x {
			t.Fatalf("mentionsForEdit(%q) = %q", x, got)
		}
	})
}
