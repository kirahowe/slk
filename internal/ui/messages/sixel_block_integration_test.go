package messages

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	stdimage "image"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/core/blocks"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/ui/imgrender"
)

// blockImageCacheKey mirrors blockkit's unexported urlCacheKey ("BK-"
// + first 8 hex chars of the URL's SHA-1) so the test can prime the
// disk cache under the key the block path will look up.
func blockImageCacheKey(url string) string {
	sum := sha1.Sum([]byte(url))
	return "BK-" + hex.EncodeToString(sum[:8])
}

// setupBlockImageMessageModel primes the image cache for one Block Kit
// image URL and returns a Model showing a single message whose Blocks
// carry that image. Mirrors setupImageMessageModel, which covers the
// file-attachment path; this one covers the block path, whose sixel
// bytes used to be emitted as an unpositioned side-channel flush
// instead of a placement.
func setupBlockImageMessageModel(t *testing.T, protocol imgpkg.Protocol) *Model {
	t.Helper()
	cache, err := imgpkg.NewCache(t.TempDir(), 10)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	fetcher := imgpkg.NewFetcher(cache, nil)
	const url = "https://example.com/block-image.png"
	key := blockImageCacheKey(url)
	pngBytes := makeTestPNG(720, 720)
	if _, err := cache.Put(key, "png", pngBytes); err != nil {
		t.Fatalf("cache.Put: %v", err)
	}
	if _, err := fetcher.Fetch(context.Background(), imgpkg.FetchRequest{
		Key:    key,
		URL:    "unused://disk-cache-hits-skip-network",
		Target: stdimage.Pt(320, 320),
	}); err != nil {
		t.Fatalf("Fetch (memo prime): %v", err)
	}
	msg := MessageItem{
		TS:        "1700000000.000100",
		UserID:    "U1",
		UserName:  "alice",
		Text:      "look at this",
		Timestamp: "10:30 AM",
		Blocks: []blocks.Block{
			// Declared square: with CellPixels (8,16) and MaxRows 20
			// the block target comes out (40,20) cells = the (320,320)
			// pixel target the cache is primed with below.
			blocks.ImageBlock{URL: url, Alt: "a square", Width: 320, Height: 320},
		},
	}
	m := New([]MessageItem{msg}, "C123")
	m.SetImageContext(imgrender.ImageContext{
		Protocol:   protocol,
		Fetcher:    fetcher,
		CellPixels: stdimage.Pt(8, 16),
		MaxRows:    20,
	})
	return m
}

// A fully visible Block Kit image under sixel must publish a placement
// through the same pipeline as file attachments. Before the fix, the
// block path returned its OnFlush as a kitty-style flush; for sixel
// that wrote the raw DCS pixel stream to the terminal at the cursor's
// current position on every View, smearing images across the screen
// while scrolling, and published no placement at all.
func TestView_SixelBlockImage_PublishesPlacement(t *testing.T) {
	m := setupBlockImageMessageModel(t, imgpkg.ProtoSixel)
	out := m.View(60, 80)
	if strings.Contains(out, "\x1bP") {
		t.Error("sixel bytes leaked into the frame string; they must be painted out-of-band")
	}
	placements := m.SixelPlacements()
	if len(placements) != 1 {
		t.Fatalf("SixelPlacements() = %d, want 1 for a fully visible block image", len(placements))
	}
	p := placements[0]
	if len(p.Bytes) == 0 || !strings.HasPrefix(string(p.Bytes), "\x1bP") {
		t.Error("placement carries no sixel DCS payload")
	}
	if p.Rows <= 0 || p.Cols <= 0 {
		t.Errorf("placement footprint = %dx%d, want a positive erase region", p.Cols, p.Rows)
	}
}

// The halfblock fallback must cover the image's rows when the image is
// only partially visible: the block path previously never populated
// SixelRows, so there was no fallback either and a partially visible
// image rendered as blank placeholder rows.
func TestView_SixelBlockImage_PartialVisibilityUsesFallback(t *testing.T) {
	m := setupBlockImageMessageModel(t, imgpkg.ProtoSixel)
	// A viewport too short to show the whole image (~20 rows) forces
	// the partial-visibility path.
	out := m.View(14, 80)
	if strings.Contains(out, "\x1bP") {
		t.Error("sixel bytes leaked into the frame string; they must be painted out-of-band")
	}
	if len(m.SixelPlacements()) != 0 {
		t.Fatalf("SixelPlacements() = %d, want 0 for a partially visible image", len(m.SixelPlacements()))
	}
	// The fallback is halfblock text: colored half-block glyphs (▀▄)
	// replacing the plain placeholder spaces.
	if !strings.Contains(out, "▀") && !strings.Contains(out, "▄") {
		t.Error("partially visible block image did not render its halfblock fallback; the rows are blank")
	}
}
