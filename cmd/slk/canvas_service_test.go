package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	slackclient "github.com/gammons/slk/internal/slack"
)

type fakeCanvasClient struct {
	ids                 []string
	file                slackclient.CanvasFile
	err                 error
	gotChannel, gotFile string
}

func (f *fakeCanvasClient) ConversationCanvases(_ context.Context, channelID string) ([]string, error) {
	f.gotChannel = channelID
	return f.ids, f.err
}

func (f *fakeCanvasClient) Canvas(_ context.Context, fileID string) (slackclient.CanvasFile, error) {
	f.gotFile = fileID
	return f.file, f.err
}

func canvasServiceFor(c *fakeCanvasClient, team string) func(string) (canvasClient, bool) {
	return func(teamID string) (canvasClient, bool) {
		if teamID != team {
			return nil, false
		}
		return c, true
	}
}

func TestCanvasServiceParsesTheNamedWorkspacesCanvas(t *testing.T) {
	fake := &fakeCanvasClient{ids: []string{"F1"}, file: slackclient.CanvasFile{
		ID: "F1", Title: "Notes", Permalink: "https://x/docs/T1/F1", Updated: 1792610107,
		HTML: []byte(`<div class="quip-canvas-content"><h1>Notes</h1></div>`),
	}}
	svc := newCanvasService(canvasServiceFor(fake, "T1"))

	ids, err := svc.ConversationCanvases(context.Background(), "T1", "D1")
	if err != nil || len(ids) != 1 || fake.gotChannel != "D1" {
		t.Fatalf("ids, err = %v, %v (channel %q)", ids, err, fake.gotChannel)
	}
	c, err := svc.Canvas(context.Background(), "T1", "F1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Notes" || c.Permalink != "https://x/docs/T1/F1" || !c.Updated.Equal(time.Unix(1792610107, 0)) {
		t.Fatalf("canvas = %+v", c)
	}
	if got := c.Doc.Plain(); got != "Notes" {
		t.Fatalf("doc = %q", got)
	}
}

func TestCanvasServiceErrors(t *testing.T) {
	svc := newCanvasService(canvasServiceFor(&fakeCanvasClient{}, "T1"))
	if _, err := svc.Canvas(context.Background(), "T2", "F1"); err == nil {
		t.Error("unknown workspace: no error")
	}

	signIn := &fakeCanvasClient{file: slackclient.CanvasFile{Permalink: "p", HTML: []byte(`<html><form>Sign in</form></html>`)}}
	c, err := newCanvasService(canvasServiceFor(signIn, "T1")).Canvas(context.Background(), "T1", "F1")
	if err == nil || !strings.Contains(err.Error(), "session") {
		t.Errorf("sign-in page: err = %v", err)
	}
	if c.Permalink != "p" {
		t.Errorf("permalink lost on error: %+v", c)
	}

	failing := &fakeCanvasClient{err: errors.New("boom")}
	if _, err := newCanvasService(canvasServiceFor(failing, "T1")).Canvas(context.Background(), "T1", "F1"); err == nil {
		t.Error("client error swallowed")
	}
}
