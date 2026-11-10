package compose

import (
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/channelpicker"
	"github.com/gammons/slk/internal/ui/mentionpicker"
	"pgregory.net/rapid"
)

func TestPreviewOffByDefaultAndToggles(t *testing.T) {
	m := New("g")
	if m.PreviewOn() {
		t.Fatal("a new box has the draft preview on")
	}
	if !m.TogglePreview() || !m.PreviewOn() {
		t.Fatal("the first toggle did not show the preview")
	}
	if m.TogglePreview() || m.PreviewOn() {
		t.Fatal("the second toggle did not hide the preview")
	}
}

// A send (Reset) and a move to another conversation close the preview;
// binding the box to the conversation it is already on does not.
func TestPreviewClosesWithTheDraft(t *testing.T) {
	m := New("g")
	m.SetPreview(true)
	m.Reset()
	if m.PreviewOn() {
		t.Fatal("Reset left the preview on")
	}

	m.SetDraftContext("T1", "C1", "")
	m.SetPreview(true)
	m.SetDraftContext("T1", "C1", "")
	if !m.PreviewOn() {
		t.Fatal("rebinding to the same conversation closed the preview")
	}
	m.SetDraftContext("T1", "C2", "")
	if m.PreviewOn() {
		t.Fatal("moving the box to another conversation left the preview on")
	}
}

// PreviewWire is the text Enter sends, for any draft and after any
// change of mention data.
func TestPreviewWireIsTheSentText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := New("g")
		m.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
		m.SetPreview(true)
		for i := range rapid.IntRange(1, 4).Draw(t, "steps") {
			switch rapid.IntRange(0, 2).Draw(t, "change") {
			case 0:
				word := rapid.OneOf(proptest.Word(), rapid.SampledFrom([]string{"@bob", "@carol", "#ops", "@team"}))
				m.SetValue(proptest.ComposerTextWith(word, []string{"U2"}, []string{"C9"}).Draw(t, "draft"))
			case 1:
				m.SetUsers([]mentionpicker.User{{ID: "U3", DisplayName: rapid.SampledFrom([]string{"bob", "carol"}).Draw(t, "user")}})
			default:
				m.SetChannels([]channelpicker.Channel{{ID: "C9", Name: "ops"}})
				m.SetUserGroups(map[string]string{"S1": "team"})
			}
			if got, want := m.PreviewWire(), m.TranslateMentionsForSend(m.Value()); got != want {
				t.Fatalf("step %d: PreviewWire = %q, Enter sends %q", i, got, want)
			}
		}
	})
}
