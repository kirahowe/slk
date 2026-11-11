package mrkdwn

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Sentinel runes from the Unicode Private Use Area. Treated as opaque
// characters by goldmark. PUA characters can legitimately appear in
// real text (some CJK IMEs, legacy Mac fonts, icon-font copy-paste),
// so tokenize sanitizes any pre-existing occurrences in the input by
// replacing them with U+FFFD before emitting its own sentinels.
const (
	sentinelStart = '\uE000'
	sentinelEnd   = '\uE001'
)

type tokenKind int

const (
	tokUser tokenKind = iota
	tokChannel
	tokBroadcast
	tokLink
	tokEmoji
)

// token holds the original wire-form payload for one <...> match.
//
// For tokUser:      id = "U12345",        label = ""
// For tokChannel:   id = "C12345",        label = "general" (may be "")
// For tokBroadcast: id = "here" / "channel" / "subteam^S01",
//
//	label = "" or "@team" (only for subteam form)
//
// For tokLink:      id = "https://x.com", label = "Slack" (may be "")
type token struct {
	kind  tokenKind
	id    string
	label string
}

// Patterns are tried in order; first match wins. None overlap given
// their leading characters (<@, <#, <!, <h, :). Each is compiled by
// tokenPattern, so it matches only at the start of what it is given.
var (
	reUser      = tokenPattern(`<@([UW][A-Z0-9]+)>`)
	reChannel   = tokenPattern(`<#([CG][A-Z0-9]+)(?:\|([^>]*))?>`)
	reBroadcast = tokenPattern(`<!([a-z]+(?:\^[A-Za-z0-9]+)?)(?:\|([^>]*))?>`)
	reLink      = tokenPattern(`<(https?://[^|>]+)(?:\|([^>]*))?>`)
	// Slack emoji shortcode. Matches the canonical wire form
	// :name: where name uses lowercase ASCII letters, digits,
	// underscores, plus, or hyphen. Skin-tone modifiers like
	// :skin-tone-3: parse as their own emoji (matching what
	// Slack web does — adjacent emoji elements compose visually).
	reEmoji = tokenPattern(`:([a-z0-9_+-]+):`)
)

// tokenPattern compiles expr anchored at the start of the text. tokenAt
// asks whether a token starts at one byte of the input, and an
// unanchored pattern that does not match there would go on searching
// the rest of the input, once for every '<' or ':' in it: Convert
// would take time quadratic in a run of those characters. An anchored
// pattern gives up at the first byte that cannot continue a token.
func tokenPattern(expr string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + expr + `)`)
}

// tokenize replaces all Slack wire-form tokens in s with sentinel
// markers and returns the rewritten string plus an ordered table.
// The marker for table index N is the three-rune sequence
// sentinelStart, decimal digits of N, sentinelEnd.
func tokenize(s string) (string, []token) {
	// Sanitize any pre-existing PUA sentinel characters in the input
	// so they cannot be confused with sentinel markers we emit.
	if strings.ContainsAny(s, string(sentinelStart)+string(sentinelEnd)) {
		s = strings.NewReplacer(
			string(sentinelStart), "\uFFFD",
			string(sentinelEnd), "\uFFFD",
		).Replace(s)
	}

	var table []token
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		// Fast path: skip ahead until we see a candidate byte. Wire-
		// form tokens all start with '<'; emoji shortcodes start with
		// ':'. IndexAny returns the earliest of the two.
		j := strings.IndexAny(s[i:], "<:")
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+j])
		i += j

		tok, n, ok := tokenAt(s, i)
		if !ok {
			// Not a recognised token; copy the candidate byte literally.
			b.WriteByte(s[i])
			i++
			continue
		}
		table = append(table, tok)
		b.WriteRune(sentinelStart)
		b.WriteString(strconv.Itoa(len(table) - 1))
		b.WriteRune(sentinelEnd)
		i += n
	}

	return b.String(), table
}

// tokenAt reads the wire-form token or emoji shortcode starting at byte
// i of s, and its length; ok is false when none starts there.
func tokenAt(s string, i int) (tok token, n int, ok bool) {
	switch s[i] {
	case '<':
		// Try each wire-form pattern at position i.
		for _, p := range []struct {
			re   *regexp.Regexp
			kind tokenKind
		}{
			{reUser, tokUser},
			{reChannel, tokChannel},
			{reBroadcast, tokBroadcast},
			{reLink, tokLink},
		} {
			loc := p.re.FindStringSubmatchIndex(s[i:])
			if loc == nil {
				continue
			}
			// loc is relative to s[i:].
			tok := token{kind: p.kind}
			tok.id = s[i+loc[2] : i+loc[3]]
			// Optional label group only exists for patterns with 2 captures.
			if len(loc) >= 6 && loc[4] >= 0 {
				tok.label = s[i+loc[4] : i+loc[5]]
			}
			return tok, loc[1], true
		}
	case ':':
		loc := reEmoji.FindStringSubmatchIndex(s[i:])
		// "see:https://x" is text and a URL, not the emoji :https:
		// (the closing colon is the scheme's). An emoji after a URL
		// is still one: "x/a:tada:" ends the URL.
		if loc != nil && typedURLLen(s[i+1:]) == 0 {
			return token{kind: tokEmoji, id: s[i+loc[2] : i+loc[3]]}, loc[1], true
		}
	}
	return token{}, 0, false
}

// Mention is a mention token in typed text as Convert reads it, from
// byte Start to End: a user (<@U…>), channel (<#C…> or <#C…|name>),
// broadcast (<!here>) or usergroup (<!subteam^S…|@handle>) mention.
type Mention struct {
	Start, End int
	// Kind is '@' for a user, '#' for a channel and '!' for a
	// broadcast or usergroup.
	Kind byte
	// ID is the user or channel ID, or what follows "<!": "here",
	// "channel", "everyone" or "subteam^S…".
	ID    string
	Label string
}

// Mentions lists the mention tokens Convert reads in s, in order, by
// the same rules as Convert: a mention-shaped run inside a link's label
// is not one, nor is a labelled user token ("<@U1|x>"). It reads no
// markdown, so a token inside a code block, which Convert sends as
// text, is listed too.
func Mentions(s string) []Mention {
	var out []Mention
	eachToken(s, func(start int, tok token, n int) {
		if kind, mention := map[tokenKind]byte{tokUser: '@', tokChannel: '#', tokBroadcast: '!'}[tok.kind]; mention {
			out = append(out, Mention{Start: start, End: start + n, Kind: kind, ID: tok.id, Label: tok.label})
		}
	})
	return out
}

// Span is the bytes of a string from Start to End.
type Span struct{ Start, End int }

// Tokens lists, in order, the span of every token Convert reads in s: a
// user, channel, broadcast or usergroup mention, a link
// (<https://…|label>) or an emoji shortcode (:name:). These are the
// tokens tokenize takes out of the text before the markdown is parsed.
// Text that only looks like a token is not one: "<@bob>", "<!-- x -->",
// a labelled user token ("<@U1|x>"), "<mailto:…>". Like Mentions it
// reads no markdown.
func Tokens(s string) []Span {
	var out []Span
	eachToken(s, func(start int, _ token, n int) {
		out = append(out, Span{start, start + n})
	})
	return out
}

// eachToken calls fn with the start, contents and length of each token
// tokenize reads in s, in order, scanning s the way tokenize does.
func eachToken(s string, fn func(start int, tok token, n int)) {
	for i := 0; i < len(s); {
		j := strings.IndexAny(s[i:], "<:")
		if j < 0 {
			break
		}
		i += j
		tok, n, ok := tokenAt(s, i)
		if !ok {
			i++
			continue
		}
		fn(i, tok, n)
		i += n
	}
}

// parseSentinel inspects s starting at byte offset start. If a
// sentinel-wrapped numeric index lives there, returns (index,
// end-byte-offset, true). The end offset points one byte past the
// closing sentinel rune.
func parseSentinel(s string, start int) (int, int, bool) {
	if start >= len(s) {
		return 0, 0, false
	}
	r, sz := utf8.DecodeRuneInString(s[start:])
	if r != sentinelStart {
		return 0, 0, false
	}
	digitStart := start + sz
	pos := digitStart
	for pos < len(s) {
		c := s[pos] // digits are ASCII, byte-level check is safe
		if c >= '0' && c <= '9' {
			pos++
			continue
		}
		break
	}
	if pos == digitStart {
		return 0, 0, false
	}
	if pos >= len(s) {
		return 0, 0, false
	}
	r, sz = utf8.DecodeRuneInString(s[pos:])
	if r != sentinelEnd {
		return 0, 0, false
	}
	digits := s[digitStart:pos]
	// Reject leading-zero indices: tokenize never emits them
	// (strconv.Itoa is canonical), so accepting them only widens the
	// collision-attack surface.
	if len(digits) > 1 && digits[0] == '0' {
		return 0, 0, false
	}
	idx, err := strconv.Atoi(digits)
	if err != nil {
		return 0, 0, false
	}
	return idx, pos + sz, true
}

// detokenizeText restores all sentinel markers in s back to their
// original Slack wire-form tokens. Used to build the mrkdwn fallback
// (where mentions stay as <@U123>).
func detokenizeText(s string, table []token) string {
	if len(table) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		idx, end, ok := parseSentinel(s, i)
		if !ok {
			r, sz := utf8.DecodeRuneInString(s[i:])
			if sz == 0 {
				break
			}
			b.WriteRune(r)
			i += sz
			continue
		}
		if idx < 0 || idx >= len(table) {
			// Index out of range — emit raw bytes literally as a
			// safety fallback. Should never happen in practice.
			b.WriteString(s[i:end])
			i = end
			continue
		}
		b.WriteString(wireForm(table[idx]))
		i = end
	}
	return b.String()
}

// wireForm reconstructs the original <...> Slack wire token.
func wireForm(t token) string {
	switch t.kind {
	case tokUser:
		return "<@" + t.id + ">"
	case tokChannel:
		if t.label == "" {
			return "<#" + t.id + ">"
		}
		return "<#" + t.id + "|" + t.label + ">"
	case tokBroadcast:
		if t.label == "" {
			return "<!" + t.id + ">"
		}
		return "<!" + t.id + "|" + t.label + ">"
	case tokLink:
		if t.label == "" {
			return "<" + t.id + ">"
		}
		return "<" + t.id + "|" + t.label + ">"
	case tokEmoji:
		return ":" + t.id + ":"
	}
	return ""
}
