package demo

import (
	"context"
	"testing"
)

// Every demo canvas parses, and every canvas a tab or message names
// exists.
func TestDemoCanvasesLoad(t *testing.T) {
	svc := canvasService()
	ids := []string{launchCanvasAttachment().FileID}
	for _, tabs := range demoCanvasTabs {
		ids = append(ids, tabs...)
	}
	for _, id := range ids {
		c, err := svc.Canvas(context.Background(), teamLumen, id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(c.Doc.Blocks) == 0 || c.Title == "" {
			t.Fatalf("%s: empty canvas %+v", id, c)
		}
	}
}
