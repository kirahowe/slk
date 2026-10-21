// Package canvas parses the HTML Slack serves for a canvas file (the
// body of its url_private_download) into a small document model the
// TUI can render. It does no I/O.
//
// The markup is Quip's: a <div class="quip-canvas-content"> root holding
// headings, p.line paragraphs, lists wrapped in
// <div data-section-style=N>, blockquotes, p.prettyprint code lines, and
// inline b / i / code / lnk / control elements. The encodings seen so far
// are recorded in docs/superpowers/specs/2026-10-21-canvas-viewer-design.md.
// Anything else is kept as its text, so an element this package does not
// know degrades to plain text instead of vanishing.
package canvas

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Document is a parsed canvas: one Block per line Slack draws.
type Document struct {
	Blocks []Block
}

// BlockKind says how a block is drawn.
type BlockKind int

const (
	Paragraph BlockKind = iota
	Heading
	ListItem
	Quote
	Code
)

// ListKind is the marker a list item gets.
type ListKind int

const (
	Bullet ListKind = iota
	Numbered
)

// Block is one line of the canvas, before wrapping.
type Block struct {
	Kind BlockKind
	// Level is the heading level, 1 to 3 (Heading only).
	Level int
	// List, Depth and Number describe a ListItem: its marker, its
	// nesting (0 for the outermost list) and, for Numbered, its
	// position.
	List   ListKind
	Depth  int
	Number int
	// Continued marks a ListItem holding text that follows a nested
	// list inside the same <li>; it is drawn indented, without a marker.
	Continued bool
	Spans     []Span
}

// ChipKind says what an inline chip (a <control> element) stands for.
type ChipKind int

const (
	NoChip ChipKind = iota
	// DateChip is a date, held only as display text ("October 21st").
	DateChip
	// UserChip is a user mention; Span.ID is the user ID and Span.Text
	// the raw "@U…" Slack serves.
	UserChip
)

// Span is a run of text with the same marks.
type Span struct {
	Text               string
	Bold, Italic, Code bool
	Link               string
	Chip               ChipKind
	ID                 string
}

// ErrNotCanvas is returned for HTML without the canvas root. Slack's
// login page is HTML too, so a download that is not a canvas must not be
// mistaken for an empty one.
var ErrNotCanvas = errors.New("canvas: not canvas HTML")

// Plain is the document's text, one line per block.
func (d Document) Plain() string {
	var b strings.Builder
	for i, blk := range d.Blocks {
		if i > 0 {
			b.WriteByte('\n')
		}
		for _, s := range blk.Spans {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

// Parse parses a canvas download.
func Parse(src string) (Document, error) {
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return Document{}, err
	}
	content := findRoot(root)
	if content == nil {
		return Document{}, ErrNotCanvas
	}
	p := &parser{}
	p.blocks(content, scope{kind: Paragraph})
	return Document{Blocks: p.out}, nil
}

func findRoot(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Div && hasClass(n, "quip-canvas-content") {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := findRoot(c); r != nil {
			return r
		}
	}
	return nil
}

// scope is what a run of inline content becomes when it is flushed.
type scope struct {
	kind  BlockKind
	level int
	list  ListKind
	depth int
	quote bool
}

type parser struct {
	out []Block
	// counters holds the next number per depth for the numbered list
	// being read; a new list section starts it afresh.
	counters []int
}

// blocks reads n's children as block content. Inline children collect
// into a run, which a block child or the end of n flushes as a block of
// scope s. A whitespace-only run is dropped.
func (p *parser) blocks(n *html.Node, s scope) {
	p.blocksFrom(n, s, false)
}

// blocksFrom is blocks with first: when set, n's first run is emitted
// even if empty (an empty <p> is a blank line, an empty <li> a bare
// marker).
func (p *parser) blocksFrom(n *html.Node, s scope, first bool) {
	var run []Span
	emitted := false
	flush := func() {
		mustEmit := first && !emitted
		if !mustEmit && blank(run) {
			run = nil
			return
		}
		blk := Block{Kind: s.kind, Level: s.level, Spans: trim(run)}
		if s.kind == ListItem {
			blk.List, blk.Depth = s.list, s.depth
			if emitted {
				blk.Continued = true
			} else if s.list == Numbered {
				blk.Number = p.number(s.depth)
			}
		}
		p.out = append(p.out, blk)
		emitted = true
		run = nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !isBlock(c) {
			run = p.inline(c, Span{}, run)
			continue
		}
		flush()
		p.block(c, s)
	}
	flush()
}

func (p *parser) block(n *html.Node, s scope) {
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.Data[1] - '0')
		if level > 3 {
			level = 3
		}
		p.blocksFrom(n, scope{kind: Heading, level: level}, true)
	case atom.P:
		kind := Paragraph
		switch {
		case hasClass(n, "prettyprint"):
			kind = Code
		case s.quote:
			kind = Quote
		}
		p.blocksFrom(n, scope{kind: kind, quote: s.quote}, true)
	case atom.Blockquote:
		p.blocks(n, scope{kind: Quote, quote: true})
	case atom.Ul, atom.Ol:
		depth := 0
		if s.kind == ListItem {
			depth = s.depth + 1
		}
		p.blocks(n, scope{kind: ListItem, list: s.list, depth: depth, quote: s.quote})
	case atom.Li:
		if s.kind != ListItem {
			s = scope{kind: ListItem, quote: s.quote}
		}
		if v, ok := attr(n, "value"); ok {
			if num, err := strconv.Atoi(v); err == nil {
				p.setNumber(s.depth, num)
			}
		}
		// The li's own text is at s.depth; a list nested inside it goes
		// one deeper, which the Ul case derives from this scope.
		p.blocksFrom(n, s, true)
	case atom.Div:
		if style, ok := attr(n, "data-section-style"); ok {
			p.counters = nil
			list := Bullet
			if style == "6" {
				list = Numbered
			}
			// The section's <ul> sees a non-list scope, so it is depth 0.
			p.blocks(n, scope{kind: Paragraph, list: list, quote: s.quote})
			return
		}
		p.blocks(n, s)
	default:
		p.blocks(n, s)
	}
}

// number returns the next number at depth and resets deeper levels, so a
// nested list restarts at 1 under each parent item.
func (p *parser) number(depth int) int {
	for len(p.counters) <= depth {
		p.counters = append(p.counters, 1)
	}
	n := p.counters[depth]
	p.counters[depth] = n + 1
	p.counters = p.counters[:depth+1]
	return n
}

func (p *parser) setNumber(depth, n int) {
	for len(p.counters) <= depth {
		p.counters = append(p.counters, 1)
	}
	p.counters[depth] = n
}

// inline appends n's text to run, carrying the marks in m. Any element
// inside inline content is read as inline, so its text keeps its place.
func (p *parser) inline(n *html.Node, m Span, run []Span) []Span {
	switch n.Type {
	case html.TextNode:
		if t := clean(n.Data); t != "" {
			s := m
			s.Text = t
			run = append(run, s)
		}
		return run
	case html.ElementNode:
	default:
		return run
	}
	switch n.DataAtom {
	case atom.B, atom.Strong:
		m.Bold = true
	case atom.I, atom.Em:
		m.Italic = true
	case atom.Code:
		m.Code = true
	case atom.A:
		if href, ok := attr(n, "href"); ok {
			m.Link = safeLink(href)
		}
	case atom.Br:
		return run
	}
	switch n.Data {
	case "lnk":
		if href, ok := attr(n, "href"); ok {
			m.Link = safeLink(href)
		}
	case "control":
		return append(run, chip(n, m))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		run = p.inline(c, m, run)
	}
	return run
}

// chip reads a <control> element as one span. A user mention is an <a>
// holding "@U…"; anything else seen so far is a date's display text.
func chip(n *html.Node, m Span) Span {
	s := m
	s.Text = clean(textOf(n))
	s.Chip = DateChip
	if a := firstChild(n, atom.A); a != nil {
		if id, ok := strings.CutPrefix(strings.TrimSpace(textOf(a)), "@"); ok && isUserID(id) {
			s.Chip, s.ID = UserChip, id
		}
	}
	return s
}

func isUserID(s string) bool {
	if len(s) < 2 || (s[0] != 'U' && s[0] != 'W') {
		return false
	}
	for _, r := range s[1:] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// safeLink keeps a link only if it is a well-formed URL with a scheme a
// terminal may be asked to open. It is written into an OSC 8 sequence, so
// a control character would let the canvas write to the terminal.
func safeLink(href string) string {
	if strings.IndexFunc(href, unicode.IsControl) >= 0 {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return href
	}
	return ""
}

// clean maps whitespace controls to spaces and drops every other control
// character, so canvas text cannot carry a terminal escape sequence.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t' || r == '\r' || r == '\f' || r == '\v':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}

func isBlock(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.P,
		atom.Div, atom.Ul, atom.Ol, atom.Li, atom.Blockquote, atom.Pre,
		atom.Table, atom.Thead, atom.Tbody, atom.Tr, atom.Td, atom.Th, atom.Hr:
		return true
	}
	return false
}

func blank(run []Span) bool {
	for _, s := range run {
		if strings.TrimSpace(s.Text) != "" {
			return false
		}
	}
	return true
}

// trim drops the whitespace at the ends of a block, which is markup
// indentation, not content.
func trim(run []Span) []Span {
	for len(run) > 0 {
		run[0].Text = strings.TrimLeftFunc(run[0].Text, unicode.IsSpace)
		if run[0].Text != "" {
			break
		}
		run = run[1:]
	}
	for len(run) > 0 {
		last := len(run) - 1
		run[last].Text = strings.TrimRightFunc(run[last].Text, unicode.IsSpace)
		if run[last].Text != "" {
			break
		}
		run = run[:last]
	}
	return run
}

func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func firstChild(n *html.Node, a atom.Atom) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == a {
			return c
		}
		if f := firstChild(c, a); f != nil {
			return f
		}
	}
	return nil
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func hasClass(n *html.Node, class string) bool {
	v, _ := attr(n, "class")
	for _, c := range strings.Fields(v) {
		if c == class {
			return true
		}
	}
	return false
}
