package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gammons/slk/internal/canvas"
	"github.com/gammons/slk/internal/core"
	slackclient "github.com/gammons/slk/internal/slack"
)

// canvasClient is the part of a workspace's *slackclient.Client the
// canvas viewer reads through.
type canvasClient interface {
	ConversationCanvases(ctx context.Context, channelID string) ([]string, error)
	Canvas(ctx context.Context, fileID string) (slackclient.CanvasFile, error)
}

var _ canvasClient = (*slackclient.Client)(nil)

// newCanvasService wires core.CanvasService to the Client of the
// workspace each call names, and parses the canvas HTML.
func newCanvasService(client func(teamID string) (canvasClient, bool)) core.CanvasService {
	return core.NewCanvasService(core.CanvasServiceFuncs{
		ConversationCanvases: func(ctx context.Context, teamID, channelID string) ([]string, error) {
			c, ok := client(teamID)
			if !ok {
				return nil, fmt.Errorf("workspace %s is not connected", teamID)
			}
			return c.ConversationCanvases(ctx, channelID)
		},
		Canvas: func(ctx context.Context, teamID, fileID string) (core.Canvas, error) {
			c, ok := client(teamID)
			if !ok {
				return core.Canvas{}, fmt.Errorf("workspace %s is not connected", teamID)
			}
			f, err := c.Canvas(ctx, fileID)
			out := core.Canvas{FileID: fileID, Title: f.Title, Permalink: f.Permalink}
			if f.Updated > 0 {
				out.Updated = time.Unix(f.Updated, 0)
			}
			if err != nil {
				return out, err
			}
			doc, err := canvas.Parse(string(f.HTML))
			if errors.Is(err, canvas.ErrNotCanvas) {
				// Slack's sign-in page is what an expired session gets.
				return out, errors.New("the download was not a canvas; the Slack session may have expired")
			}
			if err != nil {
				return out, err
			}
			out.Doc = doc
			return out, nil
		},
	})
}
