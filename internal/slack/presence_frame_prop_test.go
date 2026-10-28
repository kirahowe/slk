package slackclient

import (
	"encoding/json"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// presenceFrameFit picks the longest prefix whose presence frame (either
// type) fits the gateway's limit: that prefix's frame fits, and one more
// ID would not.
func TestPresenceFrameFitIsTheLongestPrefixThatFits(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		typ := rapid.SampledFrom([]string{presenceSubType, presenceQueryType}).Draw(t, "type")
		ids := rapid.SliceOfN(proptest.ID("U"), 0, 600).Draw(t, "ids")
		n := presenceFrameFit(typ, ids)
		frame, _ := json.Marshal(presenceFrame(typ, ids[:n]))
		if len(frame) > maxClientFrame {
			t.Fatalf("fit %d of %d ids, frame is %d bytes", n, len(ids), len(frame))
		}
		if n < len(ids) {
			next, _ := json.Marshal(presenceFrame(typ, ids[:n+1]))
			if len(next) <= maxClientFrame {
				t.Fatalf("fit %d of %d ids, but %d fit in %d bytes", n, len(ids), n+1, len(next))
			}
		}
	})
}
