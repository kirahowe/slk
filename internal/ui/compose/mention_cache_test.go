package compose

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/ui/channelpicker"
	"github.com/gammons/slk/internal/ui/mentionpicker"
)

// Every setter that feeds translation reaches the next send. The
// mention list is cached, so this guards its invalidation.
func TestTranslateSeesMentionDataChanges(t *testing.T) {
	m := New("general")
	const draft = "hi @bob #ops @team"
	if got := m.TranslateMentionsForSend(draft); got != draft {
		t.Fatalf("no mention data: got %q, want it unchanged", got)
	}
	m.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
	if got, want := m.TranslateMentionsForSend(draft), "hi <@U2> #ops @team"; got != want {
		t.Fatalf("after SetUsers: got %q, want %q", got, want)
	}
	m.SetChannels([]channelpicker.Channel{{ID: "C9", Name: "ops"}})
	if got, want := m.TranslateMentionsForSend(draft), "hi <@U2> <#C9|ops> @team"; got != want {
		t.Fatalf("after SetChannels: got %q, want %q", got, want)
	}
	m.SetUserGroups(map[string]string{"S1": "team"})
	if got, want := m.TranslateMentionsForSend(draft), "hi <@U2> <#C9|ops> <!subteam^S1|@team>"; got != want {
		t.Fatalf("after SetUserGroups: got %q, want %q", got, want)
	}
	m.SetUsers(nil)
	if got, want := m.TranslateMentionsForSend(draft), "hi @bob <#C9|ops> <!subteam^S1|@team>"; got != want {
		t.Fatalf("after SetUsers(nil): got %q, want %q", got, want)
	}
	if got, want := m.MentionsForEdit("<@U2> <#C9|ops>"), "<@U2> #ops"; got != want {
		t.Fatalf("MentionsForEdit after SetUsers(nil): got %q, want %q", got, want)
	}
}

// The list is built once per change of mention data, and every change
// counts in mentionGen.
func TestMentionListBuiltOncePerChange(t *testing.T) {
	m := New("general")
	m.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
	gen := m.mentionGen
	a, b := m.mentionList(), m.mentionList()
	if &a[0] != &b[0] {
		t.Fatal("mentionList rebuilt the list with no change in mention data")
	}
	for name, change := range map[string]func(){
		"SetUsers":      func() { m.SetUsers([]mentionpicker.User{{ID: "U3", DisplayName: "carol"}}) },
		"SetChannels":   func() { m.SetChannels([]channelpicker.Channel{{ID: "C9", Name: "ops"}}) },
		"SetUserGroups": func() { m.SetUserGroups(map[string]string{"S1": "team"}) },
	} {
		change()
		if m.mentionGen == gen {
			t.Errorf("%s did not count as a change of mention data", name)
		}
		gen = m.mentionGen
		if c := m.mentionList(); &c[0] == &a[0] {
			t.Errorf("mentionList kept the old list after %s", name)
		}
		a = m.mentionList()
	}
}

func BenchmarkTranslateMentionsForSend(b *testing.B) {
	m := New("general")
	users := make([]mentionpicker.User, 20000)
	for i := range users {
		users[i] = mentionpicker.User{ID: fmt.Sprintf("U%05d", i), DisplayName: fmt.Sprintf("person.%d", i)}
	}
	m.SetUsers(users)
	draft := strings.Repeat("- ship **v0.28** on Tuesday with @person.42 and [the notes](https://example.com/x)\n", 25)
	for b.Loop() {
		m.TranslateMentionsForSend(draft)
	}
}
