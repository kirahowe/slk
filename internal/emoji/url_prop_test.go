package emoji

import (
	"testing"

	"pgregory.net/rapid"
)

// aliasChainNames draws n distinct emoji names, each either a workspace
// name of its own or a built-in shortcode (knownNames), so a chain can
// pass through names a built-in also has.
func aliasChainNames(t *rapid.T, n int) []string {
	return rapid.SliceOfNDistinct(rapid.OneOf(
		rapid.SampledFrom(knownNames),
		rapid.StringMatching(`ws_[a-z]{1,6}`),
	), n, n, func(s string) string { return s }).Draw(t, "names")
}

// A custom emoji always wins over a built-in of the same name, at every
// hop: a chain of aliases n0 -> n1 -> ... -> nk through workspace
// entries that ends at an image resolves to that image, whichever of the
// names are also built-in shortcodes.
func TestBuildCustomEmojiURLCustomWinsAtEveryHop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := aliasChainNames(t, rapid.IntRange(1, maxAliasHops).Draw(t, "len"))
		customs := map[string]string{}
		for i := 0; i < len(names)-1; i++ {
			customs[names[i]] = customEmojiAliasPrefix + names[i+1]
		}
		const img = "https://emoji.example/end.png"
		customs[names[len(names)-1]] = img
		if got, ok := BuildCustomEmojiURL(names[0], customs); !ok || got != img {
			t.Fatalf("chain %v resolved to (%q, %v), want the workspace image", names, got, ok)
		}
	})
}

// A chain of workspace aliases that ends at a built-in no workspace
// entry names resolves to that built-in's image.
func TestBuildCustomEmojiURLChainEndingInBuiltin(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := aliasChainNames(t, rapid.IntRange(1, maxAliasHops).Draw(t, "len"))
		builtin := rapid.SampledFrom(knownNames).Filter(func(s string) bool {
			for _, n := range names {
				if n == s {
					return false
				}
			}
			return true
		}).Draw(t, "builtin")
		customs := map[string]string{}
		for i := range names {
			next := builtin
			if i+1 < len(names) {
				next = names[i+1]
			}
			customs[names[i]] = customEmojiAliasPrefix + next
		}
		cps, _ := CodepointsForShortcode(builtin)
		if got, ok := BuildCustomEmojiURL(names[0], customs); !ok || got != BuildStandardEmojiURL(cps) {
			t.Fatalf("chain %v -> %s resolved to (%q, %v), want the built-in", names, builtin, got, ok)
		}
	})
}

// For any workspace map: resolution terminates (an alias cycle is a
// miss), a name the workspace lacks is a miss, and a built-in image is
// returned only for a built-in the workspace has no entry for.
func TestBuildCustomEmojiURLAnyMap(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := aliasChainNames(t, rapid.IntRange(1, 8).Draw(t, "len"))
		customs := map[string]string{}
		for _, n := range names {
			if rapid.Bool().Draw(t, "has") {
				if rapid.Bool().Draw(t, "alias") {
					customs[n] = customEmojiAliasPrefix + rapid.SampledFrom(names).Draw(t, "target")
				} else {
					customs[n] = "https://emoji.example/" + n + ".png"
				}
			}
		}
		name := rapid.SampledFrom(names).Draw(t, "name")
		got, ok := BuildCustomEmojiURL(name, customs)
		if _, has := customs[name]; !has && ok {
			t.Fatalf("%s is not a workspace emoji, yet resolved to %q", name, got)
		}
		if !ok {
			return
		}
		// Built-in aliases share an image (+1 and thumbsup), so a built-in
		// image is wrong only when no name outside the workspace has it.
		builtinOutside := false
		for _, n := range names {
			if cps, isBuiltin := CodepointsForShortcode(n); isBuiltin && got == BuildStandardEmojiURL(cps) {
				if _, has := customs[n]; !has {
					builtinOutside = true
				}
			}
		}
		for b := range customs {
			if cps, isBuiltin := CodepointsForShortcode(b); isBuiltin && got == BuildStandardEmojiURL(cps) && !builtinOutside {
				t.Fatalf("resolved to built-in %s's image though the workspace has %s (map %v)", b, b, customs)
			}
		}
	})
}

// An alias cycle through workspace entries is a miss, even when every
// name in it is also a built-in.
func TestBuildCustomEmojiURLCycle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := aliasChainNames(t, rapid.IntRange(1, 6).Draw(t, "len"))
		customs := map[string]string{}
		for i, n := range names {
			customs[n] = customEmojiAliasPrefix + names[(i+1)%len(names)]
		}
		if got, ok := BuildCustomEmojiURL(names[0], customs); ok {
			t.Fatalf("cycle %v resolved to %q", names, got)
		}
	})
}
