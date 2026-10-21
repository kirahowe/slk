package canvas

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

// scratch.html is a canvas made for this purpose and fetched from Slack
// unchanged (see the design doc): one of each element it could hold.
func TestParseScratchCanvas(t *testing.T) {
	src, err := os.ReadFile("testdata/scratch.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	txt := func(s string) []Span { return []Span{{Text: s}} }
	want := []Block{
		{Kind: Heading, Level: 1, Spans: txt("This is a test canvas")},
		{Kind: Heading, Level: 1, Spans: txt("Heading 1")},
		{Kind: Heading, Level: 2, Spans: txt("Heading 2")},
		{Kind: Heading, Level: 3, Spans: txt("Heading 3")},
		{Kind: ListItem, List: Bullet, Spans: txt("a list item")},
		{Kind: ListItem, List: Bullet, Spans: txt("another list item")},
		{Kind: Paragraph},
		{Kind: ListItem, List: Numbered, Number: 1, Spans: txt("an ordered list")},
		{Kind: ListItem, List: Numbered, Number: 2, Spans: txt("another list item")},
		{Kind: Paragraph},
		{Kind: Quote, Spans: txt("block quote?")},
		{Kind: Code, Spans: txt("code")},
		{Kind: Paragraph, Spans: []Span{{Text: "some inline code", Code: true}}},
		{Kind: Paragraph},
		{Kind: Paragraph, Spans: []Span{{Text: "@U0123ABCDEF", Chip: UserChip, ID: "U0123ABCDEF"}}},
		{Kind: Paragraph},
		{Kind: Paragraph, Spans: []Span{{Text: "bold text", Bold: true}}},
		{Kind: Paragraph, Spans: []Span{{Text: "italic text", Italic: true}}},
		{Kind: Paragraph, Spans: []Span{{Text: "October 21st", Chip: DateChip}}},
		{Kind: Paragraph, Spans: []Span{{Text: "October 22nd", Chip: DateChip}}},
		{Kind: Paragraph},
		{Kind: Paragraph, Spans: []Span{{Text: "https://example.com", Link: "https://example.com"}}},
		{Kind: Paragraph},
	}
	if len(doc.Blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d:\n%#v", len(doc.Blocks), len(want), doc.Blocks)
	}
	for i := range want {
		if !reflect.DeepEqual(norm(doc.Blocks[i]), norm(want[i])) {
			t.Errorf("block %d:\n got %#v\nwant %#v", i, doc.Blocks[i], want[i])
		}
	}
}

// norm makes an empty span list and a nil one compare equal.
func norm(b Block) Block {
	if len(b.Spans) == 0 {
		b.Spans = nil
	}
	return b
}

// The nesting a real 1:1 canvas uses: a <ul> directly inside the <ul>
// is one level deeper, and numbering restarts under each parent.
func TestParseNestedLists(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content">` +
		`<div data-section-style='6'><ul>` +
		`<li value='1'><span>one</span><br/></li>` +
		`<ul><li><span>one.a</span></li><li><span>one.b</span></li></ul>` +
		`<li><span>two</span></li>` +
		`<ul><li><span>two.a</span></li></ul>` +
		`</ul></div>` +
		`<div data-section-style='5'><ul><li>x</li><ul><li>y</li></ul></ul></div>` +
		`</div>`)
	if err != nil {
		t.Fatal(err)
	}
	type item struct {
		text   string
		list   ListKind
		depth  int
		number int
	}
	want := []item{
		{"one", Numbered, 0, 1}, {"one.a", Numbered, 1, 1}, {"one.b", Numbered, 1, 2},
		{"two", Numbered, 0, 2}, {"two.a", Numbered, 1, 1},
		{"x", Bullet, 0, 0}, {"y", Bullet, 1, 0},
	}
	if len(doc.Blocks) != len(want) {
		t.Fatalf("got %d blocks: %#v", len(doc.Blocks), doc.Blocks)
	}
	for i, w := range want {
		b := doc.Blocks[i]
		got := item{b.Spans[0].Text, b.List, b.Depth, b.Number}
		if b.Kind != ListItem || got != w {
			t.Errorf("block %d = %v %+v, want %+v", i, b.Kind, got, w)
		}
	}
}

// Text after a list nested inside an <li> stays where it was, as a
// marker-less continuation of the item.
func TestParseTextAfterNestedListContinuesItem(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content"><ul><li>a<ul><li>b</li></ul>c</li></ul></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Plain(); got != "a\nb\nc" {
		t.Fatalf("Plain = %q", got)
	}
	if c := doc.Blocks[2]; !c.Continued || c.Depth != 0 {
		t.Fatalf("continuation = %+v", c)
	}
}

// Slack's login page is HTML too; it must not read as an empty canvas.
func TestParseRejectsHTMLWithoutCanvasRoot(t *testing.T) {
	_, err := Parse(`<html><body><form>Sign in</form></body></html>`)
	if !errors.Is(err, ErrNotCanvas) {
		t.Fatalf("err = %v, want ErrNotCanvas", err)
	}
}

func TestParseDropsUnsafeLinksAndControlCharacters(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content"><p>` +
		`<lnk href="javascript:alert(1)">js</lnk>` +
		`<lnk href="https://x.test/&#x1b;]8;;">esc</lnk>` +
		`<a href="mailto:a@b.test">mail</a>` +
		"a&#x1b;[31mb" +
		`</p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	spans := doc.Blocks[0].Spans
	if spans[0].Link != "" || spans[1].Link != "" {
		t.Errorf("unsafe links kept: %q, %q", spans[0].Link, spans[1].Link)
	}
	if spans[2].Link != "mailto:a@b.test" {
		t.Errorf("mailto link = %q", spans[2].Link)
	}
	if spans[3].Text != "a[31mb" {
		t.Errorf("text = %q, want the ESC dropped", spans[3].Text)
	}
}

func TestMentionNeedsAUserID(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content"><p><control><a>@here</a></control></p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if s := doc.Blocks[0].Spans[0]; s.Chip != DateChip || s.ID != "" {
		t.Fatalf("span = %+v, want a plain chip", s)
	}
}

// A <br> ends a line: "one<br>two" is two lines, not "onetwo".
func TestParseLineBreaks(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content"><p class="line">one<br/>two</p>` +
		`<ul><li>item<br/>more</li></ul><p><b>a<br>b</b></p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Plain(); got != "one\ntwo\nitem\nmore\na b" {
		t.Fatalf("Plain = %q", got)
	}
	if more := doc.Blocks[3]; more.Kind != ListItem || !more.Continued {
		t.Fatalf("text after <br> in an item = %+v, want a continuation", more)
	}
}

// Markup whitespace collapses as a browser draws it; a code line keeps
// its indentation and non-breaking spaces stay.
func TestParseWhitespace(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content">` +
		`<p class="line">a      <b>b</b>&nbsp;&nbsp;c</p>` +
		`<p class="prettyprint line">    indented()  // two</p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Plain(); got != "a b\u00a0\u00a0c\n    indented()  // two" {
		t.Fatalf("Plain = %q", got)
	}
}

func TestParseOrderedListAndHiddenText(t *testing.T) {
	doc, err := Parse(`<div class="quip-canvas-content"><ol><li>x</li><li>y</li></ol>` +
		`<script>alert(1)</script><style>p{}</style><p>shown</p></div>`)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Plain(); got != "x\ny\nshown" {
		t.Fatalf("Plain = %q", got)
	}
	if b := doc.Blocks[1]; b.List != Numbered || b.Number != 2 {
		t.Fatalf("second <ol> item = %+v", b)
	}
}
