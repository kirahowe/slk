package demo

import (
	"context"
	"fmt"

	"github.com/gammons/slk/internal/canvas"
	"github.com/gammons/slk/internal/core"
)

// Canvases for the canvas viewer: Priya's DM carries a 1:1 notes canvas
// tab, and the launch group DM shares a checklist canvas in a message.
// The markup is the Quip HTML Slack serves for a canvas download.
const (
	notesCanvasID  = "F0NOTES"
	launchCanvasID = "F0LAUNCH"
)

var demoCanvasTabs = map[string][]string{dmPriya: {notesCanvasID}}

var demoCanvases = map[string]struct{ title, html string }{
	notesCanvasID: {"Priya <> Alex", `<div class="quip-canvas-content">` +
		`<h1>Priya &lt;&gt; Alex</h1>` +
		`<p class="line">Topics:</p>` +
		`<div data-section-style='5'><ul><li>Session bug postmortem: who writes it up</li></ul></div>` +
		`<p class="line"></p>` +
		`<p class="line"><control><a>@U0PRIYA</a></control> <control>October 21st</control></p>` +
		`<div data-section-style='5'><ul>` +
		`<li>Goals for next quarter</li>` +
		`<ul><li>Ship the <b>rate limiter</b> behind a flag</li><li>Pair with <control><a>@U0SAM</a></control> on the config UI</li></ul>` +
		`<li>Read <lnk href="https://example.com/limiter-design">the limiter design doc</lnk> before Thursday</li>` +
		`</ul></div>` +
		`<h2>Action items</h2>` +
		`<div data-section-style='6'><ul><li value='1'>Alex: draft the postmortem</li><li>Priya: review the retry budget</li></ul></div>` +
		`<blockquote><p class="line">Small PRs, merged daily.</p></blockquote>` +
		`<p class="prettyprint line">make test &amp;&amp; make lint</p>` +
		`</div>`},
	launchCanvasID: {"Launch checklist", `<div class="quip-canvas-content">` +
		`<h1>Launch checklist</h1>` +
		`<div data-section-style='6'><ul>` +
		`<li value='1'>Freeze <code>main</code> Wednesday at noon</li>` +
		`<li>Screenshots in the press kit</li>` +
		`<li>Deploy Thursday 10am</li>` +
		`<ul><li>Sam owns the deploy</li><li>Maya watches the dashboards</li></ul>` +
		`<li>Announce in #general</li>` +
		`</ul></div></div>`},
}

func launchCanvasAttachment() core.Attachment {
	return core.Attachment{
		Kind: "canvas", Name: "Launch checklist", FileID: launchCanvasID,
		URL: "https://example.com/lumen/docs/" + launchCanvasID,
	}
}

func canvasService() core.CanvasService {
	return core.NewCanvasService(core.CanvasServiceFuncs{
		ConversationCanvases: func(_ context.Context, _, channelID string) ([]string, error) {
			return demoCanvasTabs[channelID], nil
		},
		Canvas: func(_ context.Context, _, fileID string) (core.Canvas, error) {
			c, ok := demoCanvases[fileID]
			if !ok {
				return core.Canvas{}, fmt.Errorf("files.info: file_not_found")
			}
			doc, err := canvas.Parse(c.html)
			if err != nil {
				return core.Canvas{}, err
			}
			return core.Canvas{
				FileID: fileID, Title: c.title, Doc: doc,
				Permalink: "https://example.com/lumen/docs/" + fileID,
			}, nil
		},
	})
}
