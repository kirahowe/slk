package compose

import (
	"testing"

	"github.com/gammons/slk/internal/ui/channelpicker"
	"github.com/gammons/slk/internal/ui/mentionpicker"
)

func editModel() Model {
	m := New("general")
	m.SetUsers([]mentionpicker.User{
		{ID: "U1", DisplayName: "Alice", Username: "alice"},
		{ID: "U2", DisplayName: "design", Username: "design"},
	})
	m.SetChannels(append(chans(), channelpicker.Channel{ID: "C555", Name: "general.old", Type: "channel"}))
	m.SetUserGroups(map[string]string{"S1": "platform-team", "S2": "design"})
	return m
}

func TestMentionsForEdit(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"bare channel", "see <#C111>", "see #general"},
		{"labeled channel", "see <#C111|general>.", "see #general."},
		{"stale label uses the known name", "see <#C111|old-name>", "see #general"},
		{"longer name sharing a prefix", "ping <#C222> please", "ping #general-help please"},
		{"user", "hey <@U1>, see <#C333>", "hey @Alice, see #engineering"},
		// Convert reads no label on a user token, so this one is text
		// on save, and naming it would make it a mention.
		{"labeled user is text", "hey <@U1|alice>", "hey <@U1|alice>"},
		{"special", "<!here> lunch", "@here lunch"},
		{"labeled special", "<!channel|@channel> lunch", "@channel lunch"},
		{"usergroup", "cc <!subteam^S1|@platform-team>", "cc @platform-team"},
		{"unlabeled usergroup", "cc <!subteam^S1>", "cc @platform-team"},
		{"plain text untouched", "a <b> &amp; c", "a <b> &amp; c"},
		// Each of these would not translate back to the same mention.
		{"unknown channel", "see <#C999>", "see <#C999>"},
		{"unknown user", "hey <@U9>", "hey <@U9>"},
		{"usergroup a user's name wins", "cc <!subteam^S2|@design>", "cc <!subteam^S2|@design>"},
		{"glued to the next word", "<#C111>-ish", "<#C111>-ish"},
		{"would become a longer name", "<#C111>.old", "<#C111>.old"},
	}
	m := editModel()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := m.MentionsForEdit(c.in); got != c.want {
				t.Errorf("MentionsForEdit(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Editing a message without touching it sends the same mentions back,
// with channels and usergroups in the labeled form the composer writes.
func TestMentionsForEditRoundTrips(t *testing.T) {
	m := editModel()
	in := "hey <@U1> <!here> see <#C111> and <#C222|general-help> cc <!subteam^S1|@platform-team> and <!subteam^S2|@design>"
	want := "hey <@U1> <!here> see <#C111|general> and <#C222|general-help> cc <!subteam^S1|@platform-team> and <!subteam^S2|@design>"
	if got := m.TranslateMentionsForSend(m.MentionsForEdit(in)); got != want {
		t.Errorf("round trip = %q, want %q", got, want)
	}
}
