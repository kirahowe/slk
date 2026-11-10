package emoji

import (
	"reflect"
	"testing"
)

func names(es []EmojiEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Name
	}
	return out
}

func entryName(e EmojiEntry) string { return e.Name }

func entries(ns ...string) []EmojiEntry {
	out := make([]EmojiEntry, len(ns))
	for i, n := range ns {
		out[i] = EmojiEntry{Name: n}
	}
	return out
}

func TestFrecencyPosition(t *testing.T) {
	f := NewFrecency([]string{"tada", "+1", "+1::skin-tone-3", "tada", "wave::skin-tone-5"})
	for _, tc := range []struct {
		name    string
		customs map[string]string
		wantPos int
		wantOK  bool
	}{
		{"tada", nil, 0, true},     // a repeat keeps its first position
		{"+1", nil, 1, true},       // recorded name
		{"thumbsup", nil, 1, true}, // alias of a recorded name
		{"+1_tone2", nil, 2, true}, // kyokomi spelling (tone N is Slack N+1)
		{"wave", nil, 4, true},     // the base of a toned use
		{"rocket", nil, 0, false},  // never used
		// A custom emoji named like a standard alias is its own emoji.
		{"thumbsup", map[string]string{"thumbsup": "https://e/x.png"}, 0, false},
	} {
		pos, ok := f.Position(tc.name, tc.customs)
		if pos != tc.wantPos || ok != tc.wantOK {
			t.Errorf("Position(%q) = %d, %v; want %d, %v", tc.name, pos, ok, tc.wantPos, tc.wantOK)
		}
	}
	if (Frecency{}).Len() != 0 {
		t.Error("zero Frecency ranks something")
	}
	if _, ok := (Frecency{}).Position("tada", nil); ok {
		t.Error("zero Frecency ranked tada")
	}
}

// Only Slack's "::skin-tone-N" form lifts a base: a custom emoji named
// like a kyokomi tone is an emoji of its own.
func TestFrecencyCustomToneNameIsNotATone(t *testing.T) {
	customs := map[string]string{"wave_tone1": "https://e/w.png"}
	f := NewFrecency([]string{"wave_tone1"})
	if _, ok := f.Position("wave", customs); ok {
		t.Error("a custom wave_tone1 ranked the standard wave")
	}
	if pos, ok := f.Position("wave_tone1", customs); !ok || pos != 0 {
		t.Errorf("Position(wave_tone1) = %d, %v; want 0, true", pos, ok)
	}
}

func TestMatchPutsUsedEmojiFirst(t *testing.T) {
	all := entries("fire", "fire_engine", "fireworks", "campfire", "smile", "tada")
	use := NewFrecency([]string{"fireworks", "campfire", "fire_engine", "tada"})

	for _, tc := range []struct {
		name string
		q    string
		opt  MatchOptions
		want []string
	}{
		{
			name: "no history keeps alphabetical prefix order",
			q:    "fire",
			want: []string{"fire", "fire_engine", "fireworks"},
		},
		{
			name: "used prefix matches first, by frecency",
			q:    "fire",
			opt:  MatchOptions{Frecency: use},
			want: []string{"fireworks", "fire_engine", "fire"},
		},
		{
			name: "used substring matches come after used prefix matches, before unused ones",
			q:    "fire",
			opt:  MatchOptions{Frecency: use, Substring: true},
			want: []string{"fireworks", "fire_engine", "campfire", "fire"},
		},
		{
			name: "limit cuts the tail",
			q:    "fire",
			opt:  MatchOptions{Frecency: use, Substring: true, Limit: 2},
			want: []string{"fireworks", "fire_engine"},
		},
		{
			name: "empty query lists the history, then everything else",
			q:    "",
			opt:  MatchOptions{Frecency: use},
			want: []string{"fireworks", "campfire", "fire_engine", "tada", "fire", "smile"},
		},
		{
			name: "folds case",
			q:    "FIRE_",
			opt:  MatchOptions{Frecency: use},
			want: []string{"fire_engine"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := names(Match(all, entryName, tc.q, tc.opt))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Match(%q) = %v, want %v", tc.q, got, tc.want)
			}
		})
	}
}

func TestMatchShowsAUsedEmojiOnce(t *testing.T) {
	// "+1" and "thumbsup" are the same emoji; it was used as "+1".
	all := entries("+1", "rocket", "thumbsup")
	use := NewFrecency([]string{"+1"})
	got := names(Match(all, entryName, "", MatchOptions{Frecency: use}))
	if want := []string{"+1", "rocket"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Match = %v, want %v", got, want)
	}
	// Typing an alias finds the used emoji under that alias.
	got = names(Match(BuildEntries(nil), entryName, "thumbs", MatchOptions{Frecency: use}))
	if len(got) == 0 || got[0] != "thumbsup" {
		t.Errorf(`Match("thumbs") = %v, want thumbsup first`, got)
	}
}

func TestMatchOnlyOffersEmojiInEntries(t *testing.T) {
	// A custom emoji used in another workspace is not in this one's
	// entries, so it is not offered.
	use := NewFrecency([]string{"other_workspace_parrot", "tada"})
	got := names(Match(entries("rocket", "tada"), entryName, "", MatchOptions{Frecency: use}))
	if want := []string{"tada", "rocket"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Match = %v, want %v", got, want)
	}
}

// A late-alphabet query lists its prefix matches before every substring
// match, however many "a..." names contain it (gammons/slk#193: the
// reaction picker's old scan stopped at 50 candidates, most of them
// substring matches from the top of the alphabet).
func TestMatchPrefixesBeforeSubstringsPastAnyCap(t *testing.T) {
	got := Match(BuildEntries(nil), entryName, "t", MatchOptions{Substring: true, Limit: 50})
	if len(got) != 50 {
		t.Fatalf("got %d matches, want 50", len(got))
	}
	for _, e := range got {
		if e.Name[0] != 't' {
			t.Fatalf("Match(t) lists %q among the first 50; every built-in starting with t comes first", e.Name)
		}
	}
}

func TestUsed(t *testing.T) {
	customs := map[string]string{"partyparrot": "https://e/p.gif", "facepalm": "https://e/f.png"}
	for _, tc := range []struct {
		in   []string
		want []string
	}{
		{[]string{"rocket", "tada", "rocket"}, []string{"rocket", "tada"}},
		{[]string{"thumbsup"}, []string{"+1"}},
		{[]string{"+1::skin-tone-2"}, []string{"+1::skin-tone-2"}},
		{[]string{"thumbsup::skin-tone-4"}, []string{"+1::skin-tone-4"}},
		{[]string{"30", "partyparrot"}, []string{"partyparrot"}},
		// A custom emoji shadows a standard one of the same name.
		{[]string{"facepalm"}, []string{"facepalm"}},
		{[]string{"not_an_emoji_name"}, nil},
		{nil, nil},
	} {
		if got := Used(tc.in, customs); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Used(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
