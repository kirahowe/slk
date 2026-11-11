package blockkit

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/slack-go/slack"

	"github.com/gammons/slk/internal/slack/mrkdwn"
)

// RichTextToMarkdown rebuilds the composer text a person would type
// from a parsed rich_text block: the inverse of mrkdwn.Convert, for
// seeding the composer when a sent message is edited (`E`). For a
// block Convert made, sending the result through Convert again gives
// back the same block (edit_source_prop_test.go states where).
//
// Bulleted items become "- ", ordered items "N. ", and each indent
// level is four spaces so the item nests under the one above it; a
// list right under one of the same kind and level uses "* " or "N) "
// so it stays a list of its own. Links become the bare URL when the
// label is the URL, else [label](url). Styles become **bold**,
// _italic_, ~~strike~~ and `code`. Mentions stay in wire form (<@U…>,
// <#C…>, <!here>, <!subteam^S…>) for the composer to turn into names.
// Line breaks and blank lines are kept as Convert keeps them: in the
// section text.
//
// Literal text that would read as markup ("*", "- " at a line start, a
// "<!here>" written as text) is backslash-escaped, so saving keeps it
// text and cannot ping anyone. Each place with a shorter spelling (an
// escape left out, a bare URL, a styled run over several lines marked
// once) is then a choice, and a set of short spellings is kept only
// when Convert sends the same block as before, or rt itself where the
// longer spellings did not, so a plain message reads as it was typed.
//
// The choices are decided one unit at a time (see mdUnit): a paragraph,
// a list item or a quote, which Convert reads apart from the rest of
// the message. All of a unit's choices are tried short at once and the
// set is halved only where Convert refuses it, so a unit whose escapes
// can all go costs two Converts of the unit, and the work grows with
// the length of the message. The whole seed is checked once at the
// end; if it does not send the block, the choices are decided again
// against the whole text. Either way the work is bounded (shortenWork).
func RichTextToMarkdown(rt RichTextBlock) string {
	s, _ := richTextToMarkdown(rt)
	return s
}

// shortenWork bounds the work of shortening a seed, as bytes handed to
// Convert per byte of long spelling: deciding a unit's choices reads at
// most shortenWork times the unit's length, and deciding against the
// whole text at most shortenWork times the seed's. A unit whose escapes
// can all go reads itself twice. Each escape that must stay costs a few
// more reads of its unit while the halving finds it, so the bound is
// reached only in a paragraph where many must stay (one full of literal
// "*" that would pair, say), and it keeps `E`, which runs on the Update
// goroutine, from stalling there. The choices not decided when a budget
// runs out keep their long spelling, which sends the block unchanged.
const shortenWork = 64

// shortenStats is the work richTextToMarkdown did, for tests.
type shortenStats struct {
	converts, bytes int  // Convert calls and the bytes handed to them
	units           int  // units with choices
	wholeText       bool // the choices were decided against the whole text
	exhausted       bool // a budget ran out
}

func richTextToMarkdown(rt RichTextBlock) (string, shortenStats) {
	s := &shortener{segs: layout(rt)}
	choices := false
	for _, u := range s.units() {
		if u.n+u.nb > 0 {
			choices = true
			s.stats.units++
		}
	}
	seed := s.text()
	if !choices {
		return seed, s.stats
	}
	s.target = elementsForm(rt.Elements)
	current := s.convert(seed)
	if current == s.target {
		s.byUnit()
		if final := s.text(); s.convert(final) == s.target {
			return final, s.stats
		}
		s.reset()
	}
	s.byWholeText(current)
	return s.text(), s.stats
}

// shortener decides the choices of a seed laid out as segments.
type shortener struct {
	segs   []mdSegment
	target string // elementsForm of the block
	stats  shortenStats
}

// mdSegment is a stretch of the seed: fixed text (glue), or a unit.
type mdSegment struct {
	glue string
	unit *mdUnit
}

// mdUnit is a part of the seed that Convert reads apart from the rest
// of the message: a paragraph of a section (paragraphs end at a blank
// line), a list item (from its marker; its indent is glue, put before
// each of its lines) or a quote.
// CommonMark parses the inline markup of each paragraph, item and quote
// on its own, and the text between units (separators, indents, blank
// lines) is glue no choice changes, so short spellings that send a
// unit's block unchanged send the message's unchanged. Two things reach
// past a unit, and are handled:
//
//   - Convert's tokenizer reads the whole message before any markdown,
//     and a "<#C1|", "<!here|" or "<https://" left open runs to the next
//     ">" anywhere after it, across blank lines, items and quotes. A
//     unit whose text ends inside such a token is refused (openTokenRe).
//   - A styled run kept open over line breaks (the break choices) stays
//     inside its paragraph: each paragraph is written on its own, and
//     closes its markers at its end.
//
// A line break inside a paragraph is not a boundary: emphasis, code
// spans and link brackets pair across it ("a*b\nc*d" is bold), so a
// paragraph's lines are one unit. RichTextToMarkdown checks the whole
// seed once at the end, so a case missed here costs work, not the
// block.
type mdUnit struct {
	render func(w *mdWriter) string
	// short and brk pick the short spelling of each choice and each
	// break choice; text is the unit under them, and n and nb count
	// its choices.
	short, brk []bool
	text       string
	n, nb      int
	form       string // what Convert sends for text, deciding by unit
	// indent goes before each later line of text in the seed: a list
	// item's own indent, which text, read alone, does without.
	indent string
}

// draw renders u under its choices.
func (u *mdUnit) draw() {
	w := &mdWriter{short: u.short, brk: u.brk}
	u.text = u.render(w)
	u.n, u.nb = w.n, w.nb
}

func newUnit(render func(w *mdWriter) string) mdSegment {
	u := &mdUnit{render: render}
	u.draw()
	return mdSegment{unit: u}
}

// choiceRef is one choice of a unit: choice i, or break choice i.
type choiceRef struct {
	u   *mdUnit
	i   int
	brk bool
}

func (c choiceRef) set(short bool) {
	if c.brk {
		c.u.brk[c.i] = short
	} else {
		c.u.short[c.i] = short
	}
}

func setAll(refs []choiceRef, short bool) {
	for _, c := range refs {
		c.set(short)
	}
}

// breakRefs and shortRefs list u's break choices and its other choices.
// Break choices are decided first: whether a run closes at a line break
// decides which words start a line, and so which escapes there are.
// The other choices change no count, so shortRefs is taken after.
func (u *mdUnit) breakRefs() []choiceRef {
	u.brk = make([]bool, u.nb)
	refs := make([]choiceRef, u.nb)
	for i := range refs {
		refs[i] = choiceRef{u: u, i: i, brk: true}
	}
	return refs
}

func (u *mdUnit) shortRefs() []choiceRef {
	u.short = nil
	u.draw()
	u.short = make([]bool, u.n)
	refs := make([]choiceRef, u.n)
	for i := range refs {
		refs[i] = choiceRef{u: u, i: i}
	}
	return refs
}

// decide keeps short the choices in refs that try accepts: all of them
// when it accepts them together, else each half decided in turn. try
// reports whether the choices it was given are decided (kept, or not
// drawn, or out of budget).
func decide(refs []choiceRef, try func([]choiceRef) bool) {
	if len(refs) == 0 || try(refs) || len(refs) == 1 {
		return
	}
	mid := len(refs) / 2
	decide(refs[:mid], try)
	decide(refs[mid:], try)
}

func (s *shortener) units() []*mdUnit {
	var out []*mdUnit
	for _, seg := range s.segs {
		if seg.unit != nil {
			out = append(out, seg.unit)
		}
	}
	return out
}

// text is the seed under the current choices.
func (s *shortener) text() string {
	return segmentsText(s.segs)
}

// segmentsText is segs written out, each unit under its choices.
func segmentsText(segs []mdSegment) string {
	var b strings.Builder
	for _, seg := range segs {
		if seg.unit != nil {
			b.WriteString(indentLines(seg.unit.text, seg.unit.indent))
		} else {
			b.WriteString(seg.glue)
		}
	}
	return b.String()
}

func (s *shortener) convert(text string) string {
	s.stats.converts++
	s.stats.bytes += len(text)
	return sentElements(text)
}

// budget is what is left of a shortenWork allowance.
type budget struct {
	left int
	out  bool
}

// spend takes n bytes of Convert input from b, and reports false (and
// that b is out) when there is not that much left.
func (b *budget) spend(n int) bool {
	if b.out || n > b.left {
		b.out = true
		return false
	}
	b.left -= n
	return true
}

// byUnit decides each unit's choices against the unit alone: a set of
// short spellings is kept when Convert sends the unit as before and the
// unit ends in no open token.
func (s *shortener) byUnit() {
	for _, u := range s.units() {
		if u.n+u.nb == 0 || openTokenRe.MatchString(u.text) {
			continue
		}
		b := &budget{left: shortenWork * len(u.text)}
		b.spend(len(u.text))
		u.form = s.convert(u.text)
		try := func(refs []choiceRef) bool {
			if b.out {
				return true
			}
			before := u.text
			setAll(refs, true)
			u.draw()
			ok := u.text == before // not drawn: nothing to keep
			if !ok && b.spend(len(u.text)) && !openTokenRe.MatchString(u.text) && s.convert(u.text) == u.form {
				return true
			}
			// Back to before; the counts break choices change are
			// taken again by shortRefs.
			setAll(refs, false)
			u.text = before
			return ok || b.out
		}
		decide(u.breakRefs(), try)
		decide(u.shortRefs(), try)
		if b.out {
			s.stats.exhausted = true
		}
	}
}

// reset puts every choice back to its long spelling.
func (s *shortener) reset() {
	for _, u := range s.units() {
		u.short, u.brk = nil, nil
		u.draw()
	}
}

// byWholeText decides the choices against the whole seed, by the rule
// of RichTextToMarkdown: a set is kept when the seed sends the block,
// or what it sent before (current) when that was not the block. It is
// for a seed whose long spelling does not send the block, which a unit
// alone cannot tell, and for a seed the units' decisions did not keep
// whole.
func (s *shortener) byWholeText(current string) {
	s.stats.wholeText = true
	b := &budget{left: shortenWork * len(s.text())}
	try := func(refs []choiceRef) bool {
		if b.out {
			return true
		}
		changed := s.flip(refs, true)
		if !changed {
			s.flip(refs, false)
			return true
		}
		whole := s.text()
		if b.spend(len(whole)) {
			if got := s.convert(whole); got == s.target || current != s.target && got == current {
				current = got
				return true
			}
		}
		s.flip(refs, false)
		return b.out
	}
	var refs []choiceRef
	for _, u := range s.units() {
		refs = append(refs, u.breakRefs()...)
	}
	decide(refs, try)
	refs = nil
	for _, u := range s.units() {
		refs = append(refs, u.shortRefs()...)
	}
	decide(refs, try)
	if b.out {
		s.stats.exhausted = true
	}
}

// flip sets refs to short and redraws their units, reporting whether
// any unit's text changed.
func (s *shortener) flip(refs []choiceRef, short bool) bool {
	setAll(refs, short)
	changed := false
	var last *mdUnit
	for _, c := range refs {
		if c.u == last {
			continue
		}
		last = c.u
		before := c.u.text
		c.u.draw()
		changed = changed || c.u.text != before
	}
	return changed
}

// openTokenRe matches text that ends inside a wire token Convert's
// tokenizer has begun: a channel or broadcast with a label ("<#C1|x",
// "<!here|x") or a link ("<https://x"), whose label or URL runs to the
// next ">" in the message, wherever that is.
var openTokenRe = regexp.MustCompile(`<(?:#[CG][A-Z0-9]+\||![a-z]+(?:\^[A-Za-z0-9]+)?\||https?://)[^>]*\z`)

// sentElements is the rich_text block saving s sends, as JSON.
func sentElements(s string) string {
	_, rt := mrkdwn.Convert(s)
	if rt == nil {
		return elementsForm(nil)
	}
	return elementsForm(rt.Elements)
}

// elementsForm is a block's elements as JSON, for comparing two blocks.
func elementsForm(elements []slack.RichTextElement) string {
	raw, _ := json.Marshal(elements)
	return string(raw)
}

// mdWriter renders a unit as composer text. Every place with two
// spellings (an escape or none, a link's explicit or short form) is a
// choice, numbered in render order; short[i] picks the short spelling
// of choice i, and choices past the end of short take the long one.
// Break choices (chooseBreak) are numbered apart, in brk.
type mdWriter struct {
	short, brk []bool
	n, nb      int
}

// choose is the next choice: long, or short when short asks for it.
func (w *mdWriter) choose(long, short string) string {
	i := w.n
	w.n++
	if i < len(w.short) && w.short[i] {
		return short
	}
	return long
}

// chooseBreak is the next break choice: one that keeps a run of style
// going over a line break. Break choices only work together, so a
// unit's are tried together first.
func (w *mdWriter) chooseBreak(long, short string) string {
	i := w.nb
	w.nb++
	if i < len(w.brk) && w.brk[i] {
		return short
	}
	return long
}

// layout lays the block out as the seed's segments, each choice in a
// unit, all in their long spelling.
func layout(rt RichTextBlock) []mdSegment {
	var segs []mdSegment
	var b strings.Builder
	var prev slack.RichTextElement
	prevList := -1   // indent of the list element just written; -1 after anything else
	prevAlt := false // whether it used the other markers
	for _, e := range rt.Elements {
		var part []mdSegment
		list, alt := -1, false
		switch v := e.(type) {
		case *slack.RichTextSection:
			part = paragraphs(v.Elements)
		case *slack.RichTextList:
			// A nested level sits directly under its parent list; one
			// with nothing above it to nest under starts at the margin
			// (four spaces after a paragraph are paragraph text).
			list = min(v.Indent, prevList+1)
			alt = sameList(prev, v) && prevList == list && !prevAlt
			part = listItems(v, list, alt)
		case *slack.RichTextPreformatted:
			part = []mdSegment{{glue: preformattedToMarkdown(v.Elements)}}
		case *slack.RichTextQuote:
			elements := v.Elements
			part = []mdSegment{newUnit(func(w *mdWriter) string { return w.quote(elements) })}
		}
		text := segmentsText(part)
		if strings.Trim(text, "\n") == "" {
			continue
		}
		if b.Len() > 0 {
			sep := separator(prev, e, b.String())
			segs = append(segs, mdSegment{glue: sep})
			b.WriteString(sep)
		}
		segs = append(segs, part...)
		b.WriteString(text)
		prev, prevList, prevAlt = e, list, alt
	}
	return segs
}

// paragraphs splits a section's elements at its blank lines into one
// unit per paragraph, the line breaks between them glue.
func paragraphs(elements []slack.RichTextSectionElement) []mdSegment {
	var segs []mdSegment
	var cur []slack.RichTextSectionElement
	flush := func() {
		if len(cur) > 0 {
			p := cur
			segs = append(segs, newUnit(func(w *mdWriter) string { return w.inlines(p) }))
			cur = nil
		}
	}
	for _, e := range elements {
		t, ok := e.(*slack.RichTextSectionTextElement)
		if !ok || !strings.Contains(t.Text, "\n\n") {
			cur = append(cur, e)
			continue
		}
		text, start := t.Text, 0
		for _, loc := range blankLinesRe.FindAllStringIndex(text, -1) {
			if loc[0] > start {
				cur = append(cur, withText(t, text[start:loc[0]]))
			}
			flush()
			segs = append(segs, mdSegment{glue: text[loc[0]:loc[1]]})
			start = loc[1]
		}
		if start < len(text) {
			cur = append(cur, withText(t, text[start:]))
		}
	}
	flush()
	return segs
}

// blankLinesRe matches the line breaks that end a paragraph.
var blankLinesRe = regexp.MustCompile(`\n\n+`)

// withText is t with text in place of its own.
func withText(t *slack.RichTextSectionTextElement, text string) *slack.RichTextSectionTextElement {
	c := *t
	c.Text = text
	return &c
}

// listItems lays out a list, one item per line, four spaces per indent
// level. Ordered items are numbered from Offset+1. An item is a unit
// from its marker; its indent is glue, and also goes before the item's
// later lines in the seed (mdUnit.indent).
func listItems(l *slack.RichTextList, indent int, alt bool) []mdSegment {
	pad := strings.Repeat("    ", indent)
	var segs []mdSegment
	for i, child := range l.Elements {
		var body []slack.RichTextSectionElement
		if sec, ok := child.(*slack.RichTextSection); ok {
			body = sec.Elements
		}
		marker := "- "
		if alt {
			marker = "* "
		}
		if l.Style == slack.RTEListOrdered {
			marker = strconv.Itoa(l.Offset+i+1) + ". "
			if alt {
				marker = strconv.Itoa(l.Offset+i+1) + ") "
			}
		}
		glue := pad
		if i > 0 {
			glue = "\n" + pad
		}
		if glue != "" {
			segs = append(segs, mdSegment{glue: glue})
		}
		// The item's later lines start at its text's column, after
		// the indent and the marker: a line short of it is not in the
		// item.
		item := newUnit(func(w *mdWriter) string {
			return marker + indentLines(w.inlines(body), strings.Repeat(" ", len(marker)))
		})
		item.unit.indent = pad
		segs = append(segs, item)
	}
	return segs
}

// indentLines puts indent before each line of s after the first, except
// empty ones (a blank line needs none, and spaces on it would be text).
func indentLines(s, indent string) string {
	if indent == "" || !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// separator is the text between two top-level elements. Convert puts
// the blank lines typed next to a paragraph into its section's text,
// so one line break is enough there and between a list, code block or
// quote that follows another. Two quotes in a row need a blank line,
// or they read as one; so does an ordered list not starting at 1 under
// a paragraph, which CommonMark reads as more of the paragraph, unless
// the section already ends in one (written is the text so far).
func separator(prev, next slack.RichTextElement, written string) string {
	_, prevQuote := prev.(*slack.RichTextQuote)
	_, nextQuote := next.(*slack.RichTextQuote)
	if prevQuote && nextQuote {
		return "\n\n"
	}
	_, prevSection := prev.(*slack.RichTextSection)
	_, nextSection := next.(*slack.RichTextSection)
	if prevSection && nextSection {
		// Slack draws sibling sections with no break between them;
		// as one paragraph they draw the same.
		return ""
	}
	if prevSection && !strings.HasSuffix(written, "\n") {
		if l, ok := next.(*slack.RichTextList); ok && l.Style == slack.RTEListOrdered && l.Offset > 0 {
			return "\n\n"
		}
	}
	return "\n"
}

// inlines renders the inline elements of a section, list item or quote.
//
// Bold, italic and strike are written as markers that open where a
// run of that style starts and close where it ends, so text styled
// across several elements ("**see `this`**", a bold link inside bold
// text) is marked once around all of them. A marker has to touch text,
// so spaces at the edge of a styled run go outside it. Neighbouring
// code-styled elements are written as one code span: two spans side by
// side would run their backticks together.
func (w *mdWriter) inlines(elements []slack.RichTextSectionElement) string {
	var b strings.Builder
	var open []emphasis // markers open, outermost first
	pending := ""       // whitespace not yet written, kept outside markers that close
	atStart := true     // whether b ends at a line start, past any spaces
	write := func(s string) {
		b.WriteString(s)
		atStart = lineStart(atStart, s)
	}
	// closeAtBreaks is one choice for every line break in these
	// elements: Convert keeps a run over several breaks only when
	// none of them closes it.
	closeAt := ""
	closeAtBreaks := func() bool {
		if closeAt == "" {
			closeAt = w.chooseBreak("close", "open")
		}
		return closeAt == "close"
	}
	closeTo := func(n int) {
		for len(open) > n {
			write(open[len(open)-1].marker())
			open = open[:len(open)-1]
		}
	}
	pieces := w.pieces(elements)
	for i, p := range pieces {
		keep := 0
		for keep < len(open) && p.has(open[keep]) {
			keep++
		}
		if p.render == nil { // whitespace between words
			// Whitespace ends the markers it is not styled with; a
			// line break inside styled text may also end the rest
			// there (long) or leave them open across it (short).
			if strings.Contains(p.text, "\n") && keep > 0 && closeAtBreaks() {
				keep = 0
			}
			if keep < len(open) {
				closeTo(keep)
			}
			pending += p.text
			continue
		}
		closeTo(keep)
		write(pending)
		pending = ""
		// The marker whose style lasts longest opens first, so it is
		// the last to close.
		var opening []emphasis
		for _, e := range emphases {
			if p.has(e) && !slices.Contains(open, e) {
				opening = append(opening, e)
			}
		}
		slices.SortStableFunc(opening, func(a, b emphasis) int {
			return styleRun(pieces[i:], b) - styleRun(pieces[i:], a)
		})
		for _, e := range opening {
			write(e.marker())
			open = append(open, e)
		}
		write(p.render(w, atStart))
	}
	closeTo(0)
	write(pending)
	return b.String()
}

// styleRun is how many of pieces, from the first, carry e before one
// that does not (whitespace between them aside).
func styleRun(pieces []piece, e emphasis) int {
	n := 0
	for _, p := range pieces {
		if p.render == nil {
			continue
		}
		if !p.has(e) {
			break
		}
		n++
	}
	return n
}

// emphasis is a style written as a pair of markers.
type emphasis int

const (
	bold emphasis = iota
	italic
	strike
)

// emphases is the order markers open in, outermost first.
var emphases = []emphasis{bold, italic, strike}

func (e emphasis) marker() string {
	return [...]string{"**", "_", "~~"}[e]
}

// piece is one inline element, or one word or run of whitespace of a
// text element, with its bold, italic and strike flags; render writes
// its content without them, and is nil for whitespace.
type piece struct {
	text   string
	style  *slack.RichTextSectionTextStyle
	render func(w *mdWriter, atLineStart bool) string
}

func (p piece) has(e emphasis) bool {
	if p.style == nil {
		return false
	}
	return [...]bool{p.style.Bold, p.style.Italic, p.style.Strike}[e]
}

// pieces splits elements into what inlines writes one at a time: text
// runs split into their spaces and lines (a code run keeps its own),
// neighbouring code-styled elements joined into one code span, and
// every other element whole.
func (w *mdWriter) pieces(elements []slack.RichTextSectionElement) []piece {
	var out []piece
	for i := 0; i < len(elements); i++ {
		if span, j, ok := codeRunAt(elements, i); ok {
			out = append(out, piece{text: span, style: elementStyle(elements[i]), render: fixed(span)})
			i = j - 1
			continue
		}
		e := elements[i]
		style := elementStyle(e)
		switch v := e.(type) {
		case *slack.RichTextSectionTextElement:
			if style != nil && style.Code {
				text := v.Text
				out = append(out, piece{text: text, style: style, render: func(w *mdWriter, _ bool) string {
					return w.codeLines(text)
				}})
				continue
			}
			out = append(out, textPieces(v.Text, style)...)
		case *slack.RichTextSectionLinkElement:
			url, label := v.URL, v.Text
			out = append(out, piece{text: url, style: style, render: func(w *mdWriter, _ bool) string {
				return codeIf(w.link(url, label), style)
			}})
		case *slack.RichTextSectionUnknownElement:
			if m, ok := linkMention(v); ok {
				out = append(out, piece{text: m.URL, render: func(w *mdWriter, _ bool) string {
					return w.link(m.URL, m.Text)
				}})
			}
		default:
			s := codeIf(inlineToMrkdwn(e), style)
			if s != "" {
				out = append(out, piece{text: s, style: style, render: fixed(s)})
			}
		}
	}
	return out
}

// textPieces splits literal text into its runs of whitespace and the
// words between them.
func textPieces(text string, style *slack.RichTextSectionTextStyle) []piece {
	var out []piece
	for text != "" {
		space := func(r rune) bool { return r == ' ' || r == '\n' }
		n := strings.IndexFunc(text, func(r rune) bool { return !space(r) })
		if n != 0 {
			if n < 0 {
				n = len(text)
			}
			out = append(out, piece{text: text[:n], style: style})
			text = text[n:]
			continue
		}
		if n = strings.IndexFunc(text, space); n < 0 {
			n = len(text)
		}
		word := text[:n]
		out = append(out, piece{text: word, style: style, render: func(w *mdWriter, atLineStart bool) string {
			return w.escape(word, atLineStart)
		}})
		text = text[n:]
	}
	return out
}

// fixed renders as s.
func fixed(s string) func(*mdWriter, bool) string {
	return func(*mdWriter, bool) string { return s }
}

// codeIf puts s, a wire token, in a code span when style is code:
// Convert gives a token typed inside one the code style.
func codeIf(s string, style *slack.RichTextSectionTextStyle) string {
	if style != nil && style.Code {
		return codeSpan(s)
	}
	return s
}

// codeLines writes code-styled text. Text over several lines is one
// span (short) or a span per line (long): a span holds a line break,
// but a line in it that reads as a list item ends it.
func (w *mdWriter) codeLines(text string) string {
	if !strings.Contains(text, "\n") {
		return codeText(text)
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = codeText(l)
		}
	}
	return w.chooseBreak(strings.Join(lines, "\n"), codeText(text))
}

// codeRunAt is the code span for the run of code-styled elements
// starting at i and the index after it, when there is a run of two or
// more that Convert reads back as written: the whole run, else the run
// up to its first line break.
func codeRunAt(elements []slack.RichTextSectionElement, i int) (string, int, bool) {
	j := codeRunEnd(elements, i)
	if j <= i+1 {
		return "", 0, false
	}
	if span, ok := codeRun(elements[i:j]); ok {
		return span, j, true
	}
	for k := i; k < j; k++ {
		if t, ok := elements[k].(*slack.RichTextSectionTextElement); ok && strings.Contains(t.Text, "\n") {
			if k > i+1 {
				if span, ok := codeRun(elements[i:k]); ok {
					return span, k, true
				}
			}
			break
		}
	}
	return "", 0, false
}

// codeRunEnd is the end of the run of code-styled elements, all in the
// same style, that starts at i. A broadcast inside it is part of it:
// Convert gives a broadcast no style, wherever it was typed.
func codeRunEnd(elements []slack.RichTextSectionElement, i int) int {
	first := elementStyle(elements[i])
	if first == nil || !first.Code {
		return i
	}
	j := i + 1
	for j < len(elements) {
		if _, ok := elements[j].(*slack.RichTextSectionBroadcastElement); ok {
			j++
			continue
		}
		if st := elementStyle(elements[j]); st == nil || *st != *first {
			break
		}
		j++
	}
	for j > i+1 {
		if _, ok := elements[j-1].(*slack.RichTextSectionBroadcastElement); !ok {
			break
		}
		j--
	}
	return j
}

// codeRun writes elements, all code-styled alike, as one code span of
// their text and wire tokens, when Convert reads it back as the same
// elements.
func codeRun(elements []slack.RichTextSectionElement) (string, bool) {
	var content strings.Builder
	for _, e := range elements {
		switch v := e.(type) {
		case *slack.RichTextSectionTextElement:
			content.WriteString(v.Text)
		case *slack.RichTextSectionLinkElement:
			content.WriteString(linkToMrkdwn(v.URL, v.Text))
		default:
			content.WriteString(inlineToMrkdwn(e))
		}
	}
	span := codeSpan(content.String())
	_, rt := mrkdwn.Convert(emphasize(span, elementStyle(elements[0])))
	if rt == nil || len(rt.Elements) != 1 {
		return "", false
	}
	sec, ok := rt.Elements[0].(*slack.RichTextSection)
	if !ok || elementsForm(sectionElements(sec.Elements)) != elementsForm(sectionElements(elements)) {
		return "", false
	}
	return span, true
}

// sectionElements wraps inline elements as a block's elements, for
// elementsForm.
func sectionElements(elements []slack.RichTextSectionElement) []slack.RichTextElement {
	return []slack.RichTextElement{slack.NewRichTextSection(elements...)}
}

// elementStyle is the style of an inline element that has one.
func elementStyle(e slack.RichTextSectionElement) *slack.RichTextSectionTextStyle {
	switch v := e.(type) {
	case *slack.RichTextSectionTextElement:
		return v.Style
	case *slack.RichTextSectionLinkElement:
		return v.Style
	case *slack.RichTextSectionUserElement:
		return v.Style
	case *slack.RichTextSectionChannelElement:
		return v.Style
	case *slack.RichTextSectionUserGroupElement:
		return v.Style
	case *slack.RichTextSectionEmojiElement:
		return v.Style
	}
	return nil
}

// lineStart reports whether text written after s starts a line, past
// any spaces: where "- ", "1. " and "> " are markup. after is whether
// the text before s ended at one, so a writer can keep track as it
// goes instead of reading back over the line.
func lineStart(after bool, s string) bool {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimRight(s[i+1:], " ") == ""
	}
	return after && strings.TrimRight(s, " ") == ""
}

// escape backslash-escapes each character of literal text that can
// make markup: the emphasis and code markers, brackets and parens of a
// link, "<", ">" and "|" of a wire token, ":" of an emoji shortcode or
// URL, "!" of an image, the backslash itself, and at a line start the
// list and quote markers ("-", "+", "=", and "." or ")" after digits).
// Each escape is a choice, so RichTextToMarkdown drops those that
// change nothing.
func (w *mdWriter) escape(s string, atLineStart bool) string {
	var b strings.Builder
	lead, digits := atLineStart, false
	for _, r := range s {
		special := strings.ContainsRune("\\`*_~[]()<>|:!", r) ||
			lead && strings.ContainsRune("-+=", r) ||
			digits && (r == '.' || r == ')')
		if special {
			b.WriteString(w.choose(`\`, ""))
		}
		b.WriteRune(r)
		switch {
		case r == '\n':
			lead, digits = true, false
		case r == ' ' && lead:
		case r >= '0' && r <= '9' && (lead || digits):
			lead, digits = false, true
		default:
			lead, digits = false, false
		}
	}
	return b.String()
}

// emphasize wraps s in the strike, italic and bold markers style asks
// for, bold outermost. Code is the caller's: a link can't be one.
func emphasize(s string, style *slack.RichTextSectionTextStyle) string {
	if style == nil {
		return s
	}
	if style.Strike {
		s = "~~" + s + "~~"
	}
	if style.Italic {
		s = "_" + s + "_"
	}
	if style.Bold {
		s = "**" + s + "**"
	}
	return s
}

// codeText writes literal text in code style. Convert reads wire tokens
// before markdown, so "<!here>" typed in a code span is a broadcast;
// when s holds a token Convert would read, each such token's "<" is
// written escaped outside the code span. It loses the code style but
// keeps the text, which cannot ping on save.
func codeText(s string) string {
	whole := codeSpan(s)
	loc := literalTokenRe.FindStringIndex(s)
	if loc == nil || convertsToCode(whole, s) {
		return whole
	}
	var b strings.Builder
	if loc[0] > 0 {
		b.WriteString(codeText(s[:loc[0]]))
	}
	b.WriteString(`\<`)
	if rest := s[loc[0]+1:]; rest != "" {
		b.WriteString(codeText(rest))
	}
	return b.String()
}

// convertsToCode reports whether Convert sends md, written after other
// text on a line, as one code-styled text run reading s. (At the start
// of a line a span of three backticks or more would open a code
// block.)
func convertsToCode(md, s string) bool {
	_, rt := mrkdwn.Convert("x " + md)
	if rt == nil || len(rt.Elements) != 1 {
		return false
	}
	sec, ok := rt.Elements[0].(*slack.RichTextSection)
	if !ok || len(sec.Elements) != 2 {
		return false
	}
	t, ok := sec.Elements[1].(*slack.RichTextSectionTextElement)
	return ok && t.Text == s && t.Style != nil && t.Style.Code
}

// codeSpan wraps s in a run of backticks one longer than the longest
// run inside it, padding with spaces when s starts or ends with one,
// or with a space at both ends, as CommonMark requires.
func codeSpan(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	bothSpaces := len(s) > 1 && s[0] == ' ' && s[len(s)-1] == ' ' && strings.Trim(s, " ") != ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || bothSpaces {
		s = " " + s + " "
	}
	return fence + s + fence
}

// link writes a link. Its long form is Slack's <url|label> (<url> when
// the label adds nothing), which Convert takes as typed for an http or
// https URL; any other link is [label](url), with the URL in <> when it
// has a space or a paren, and the label as written (Convert does not
// read escapes there). The short form is what a person types: the bare
// URL when the label adds nothing (or the address, for mailto:), else
// [label](url). RichTextToMarkdown keeps the short form only when
// Convert links it to the same target, so a bare URL the typed-URL rule
// would cut, or that runs into the text after it, stays explicit.
func (w *mdWriter) link(url, label string) string {
	bare := label == "" || label == url || url == "mailto:"+label
	var long string
	switch {
	case (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) &&
		!strings.ContainsAny(url, "|<>") && !strings.ContainsAny(label, "<>"):
		long = "<" + url + ">"
		if !bare {
			long = "<" + url + "|" + label + ">"
		}
	default:
		if label == "" {
			label = url
		}
		dest := url
		if strings.ContainsAny(url, " ()") {
			dest = "<" + url + ">"
		}
		long = "[" + label + "](" + dest + ")"
	}
	short := "[" + label + "](" + url + ")"
	if bare {
		short = strings.TrimPrefix(url, "mailto:")
	}
	return w.choose(long, short)
}

// LinkToMarkdown writes one link as composer text, on its own: the
// short form (bare URL, address or [label](url)) when Convert sends it
// as the same link as the long one, else the long one. See
// (*mdWriter).link.
func LinkToMarkdown(url, label string) string {
	long := (&mdWriter{}).link(url, label)
	short := (&mdWriter{short: []bool{true}}).link(url, label)
	if sentElements(short) == sentElements(long) {
		return short
	}
	return long
}

// EscapeLiteralTokens backslash-escapes every run of literal text that
// Convert would read as a wire token ("<!here>", "<@U1>",
// "<https://x|y>"): its "<", "|", ">" and the "://" of a link, so the
// composer sends it as the text it is and a mention written out as
// text cannot ping anyone when an edit is saved. For text known to be
// literal, such as Slack's text field with its escapes decoded.
func EscapeLiteralTokens(s string) string {
	return literalTokenRe.ReplaceAllStringFunc(s, func(tok string) string {
		inner := strings.NewReplacer("|", `\|`, "://", `\://`).Replace(tok[1 : len(tok)-1])
		return `\<` + inner + `\>`
	})
}

// literalTokenRe matches text shaped like a wire token Convert reads: a
// mention, broadcast or http(s) link.
var literalTokenRe = regexp.MustCompile(`<(?:[@#!]|https?://)[^<>]*>`)

// preformattedToMarkdown writes a fenced code block. The fence is one
// backtick longer than any run of three or more inside the body.
func preformattedToMarkdown(elements []slack.RichTextSectionElement) string {
	body := strings.TrimSuffix(strings.TrimPrefix(preformattedToMrkdwn(elements), "```\n"), "\n```")
	body = strings.TrimSuffix(body, "\n")
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	return fence + "\n" + body + "\n" + fence
}

// sameList reports whether l has prev's list style, so that written
// right under it with the same markers it would read as more of prev:
// CommonMark starts a new list only where the marker changes.
func sameList(prev slack.RichTextElement, l *slack.RichTextList) bool {
	p, ok := prev.(*slack.RichTextList)
	return ok && p.Style == l.Style
}

// quote writes a rich_text_quote as "> " lines.
func (w *mdWriter) quote(elements []slack.RichTextSectionElement) string {
	lines := strings.Split(strings.Trim(w.inlines(elements), "\n"), "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return strings.Join(lines, "\n")
}
