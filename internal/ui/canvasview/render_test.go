package canvasview

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/canvas"
)

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(xansi.Strip(l), " ")
	}
	return out
}

func TestRenderMarkersAndIndent(t *testing.T) {
	src := `<div class="quip-canvas-content">` +
		`<h1>Title</h1><p class="line">Topics:</p>` +
		`<div data-section-style='5'><ul><li>alpha</li><ul><li>beta</li></ul></ul></div>` +
		`<h2>Next</h2>` +
		`<div data-section-style='6'><ul><li value='1'>one</li><li>two</li></ul></div>` +
		`<blockquote><p class="line">quoted</p></blockquote>` +
		`<p class="prettyprint line">x := 1</p>` +
		`<p class="line"><control>October 21st</control> <control><a>@U1</a></control> <control><a>@U2</a></control></p>` +
		`</div>`
	doc, err := canvas.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	names := func(id string) (string, bool) {
		if id == "U1" {
			return "seth", true
		}
		return "", false
	}
	got := plainLines(Render(doc, 30, names))
	want := []string{
		"Title",
		"Topics:",
		"• alpha",
		"  ◦ beta",
		"",
		"Next",
		"1. one",
		"2. two",
		"│ quoted",
		"x := 1",
		"📅 October 21st @seth @U2",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A wrapped list item hangs under its text, not under the marker.
func TestRenderHangingIndent(t *testing.T) {
	doc := canvas.Document{Blocks: []canvas.Block{{
		Kind: canvas.ListItem, List: canvas.Numbered, Number: 12, Depth: 1,
		Spans: []canvas.Span{{Text: "aaa bbb ccc ddd"}},
	}}}
	got := plainLines(Render(doc, 14, noNames))
	want := []string{"  12. aaa bbb", "      ccc ddd"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Each word of a link carries the hyperlink on its own, so a wrap
// between words leaves no line with an unterminated OSC 8.
func TestRenderLinkWordsAreSelfContained(t *testing.T) {
	doc := canvas.Document{Blocks: []canvas.Block{{
		Spans: []canvas.Span{{Text: "one two three", Link: "https://example.com"}},
	}}}
	for _, line := range Render(doc, 8, noNames) {
		if open, close := strings.Count(line, "\x1b]8;;https"), strings.Count(line, "\x1b]8;;\x1b\\"); open != close {
			t.Fatalf("line %q opens %d links and closes %d", line, open, close)
		}
	}
}
