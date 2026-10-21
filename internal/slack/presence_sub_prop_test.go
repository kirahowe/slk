package slackclient

import (
	"encoding/json"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// presenceSubFit picks the longest prefix whose presence_sub frame fits
// the gateway's limit: that prefix's frame fits, and one more ID would
// not.
func TestPresenceSubFitIsTheLongestPrefixThatFits(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ids := rapid.SliceOfN(proptest.ID("U"), 0, 600).Draw(t, "ids")
		n := presenceSubFit(ids)
		frame, _ := json.Marshal(presenceSub(ids[:n]))
		if len(frame) > maxClientFrame {
			t.Fatalf("fit %d of %d ids, frame is %d bytes", n, len(ids), len(frame))
		}
		if n < len(ids) {
			next, _ := json.Marshal(presenceSub(ids[:n+1]))
			if len(next) <= maxClientFrame {
				t.Fatalf("fit %d of %d ids, but %d fit in %d bytes", n, len(ids), n+1, len(next))
			}
		}
	})
}
