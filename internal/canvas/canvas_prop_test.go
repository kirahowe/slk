package canvas

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"golang.org/x/net/html"
	"pgregory.net/rapid"
)

// fataler is what both *testing.T and *rapid.T offer.
type fataler interface {
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

// rootText is every shown text node under the canvas root (not script,
// style or template), in document order, cleaned the way the parser
// cleans text: what a reader of the canvas should get back, element
// types aside.
func rootText(t fataler, src string) (string, bool) {
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	content := findRoot(root)
	if content == nil {
		return "", false
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if hidden(n) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(clean(n.Data))
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(content)
	return b.String(), true
}

// checkKeepsText is the parser's property: the words of the document are
// the words of the canvas's text nodes, in order. Nothing vanishes,
// nothing is invented, and no text carries a control character.
func checkKeepsText(t fataler, src string) {
	want, ok := rootText(t, src)
	doc, err := Parse(src)
	if !ok {
		if err == nil {
			t.Fatalf("Parse accepted HTML without a canvas root: %q", src)
		}
		return
	}
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	// Word boundaries are not kept exactly: "a<b>b</b>" is one word in
	// the source and two spans in the document. Compare the words with
	// every space removed, which still catches a lost or moved word.
	join := func(s string) string { return strings.Join(strings.Fields(s), "") }
	if got := join(doc.Plain()); got != join(want) {
		t.Fatalf("text changed\n src: %q\n got: %q\nwant: %q", src, got, join(want))
	}
	for _, b := range doc.Blocks {
		for _, s := range b.Spans {
			if strings.ContainsFunc(s.Text, isControl) || strings.ContainsFunc(s.Link, isControl) {
				t.Fatalf("control character in %+v", s)
			}
		}
	}
}

func isControl(r rune) bool { return r < 0x20 || (r >= 0x7f && r < 0xa0) }

// Canvas-shaped markup: the elements Slack serves, nested at random.
func canvasHTML() *rapid.Generator[string] {
	var node func(depth int) *rapid.Generator[string]
	node = func(depth int) *rapid.Generator[string] {
		return rapid.Custom(func(t *rapid.T) string {
			if depth > 3 {
				return html.EscapeString(proptest.PlainText().Draw(t, "leaf"))
			}
			kids := rapid.SliceOfN(node(depth+1), 0, 3).Draw(t, "kids")
			inner := strings.Join(kids, "")
			tag := rapid.SampledFrom([]string{
				"", "p", `p class="line"`, `p class="prettyprint line"`, "h1", "h2", "h3",
				"blockquote", "ul", "li", `li value="3"`, "span", "b", "i", "code", "br",
				`div data-section-style="5"`, `div data-section-style="6"`,
				"control", `lnk href="https://example.com"`, "a", "table", "td", "hr",
				"ol", "script", "style",
			}).Draw(t, "tag")
			if tag == "" {
				return html.EscapeString(proptest.PlainText().Draw(t, "text")) + inner
			}
			name, _, _ := strings.Cut(tag, " ")
			if tag == "control" && rapid.Bool().Draw(t, "mention") {
				inner = "<a>@" + proptest.ID("U").Draw(t, "user") + "</a>"
			}
			return "<" + tag + ">" + inner + "</" + name + ">"
		})
	}
	return rapid.Custom(func(t *rapid.T) string {
		return `<div class="quip-canvas-content">` + node(0).Draw(t, "body") + `</div>`
	})
}

func TestParseKeepsText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		checkKeepsText(t, canvasHTML().Draw(t, "src"))
	})
}

// Numbers of consecutive items in one numbered list at one depth count
// up by one.
func TestNumberedItemsCountUp(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 12).Draw(t, "n")
		items := slices.Repeat([]string{"<li>x</li>"}, n)
		doc, err := Parse(`<div class="quip-canvas-content"><div data-section-style="6"><ul>` +
			strings.Join(items, "") + `</ul></div></div>`)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range doc.Blocks {
			if b.Number != i+1 {
				t.Fatalf("item %d numbered %d", i, b.Number)
			}
		}
	})
}

func FuzzParse(f *testing.F) {
	if src, err := os.ReadFile("testdata/scratch.html"); err == nil {
		f.Add(string(src))
	}
	f.Add(`<div class="quip-canvas-content"><ul><li>a<ul><li>b</li></ul>c</li></ul></div>`)
	f.Add(`<div class="quip-canvas-content"><p><control><a>@U1</a></control><lnk href="x">y</lnk></p></div>`)
	f.Add(`<div class="quip-canvas-content"></div><p>after</p>`)
	f.Fuzz(func(t *testing.T, src string) {
		checkKeepsText(t, src)
	})
}
