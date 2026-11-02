package slackclient

import (
	"encoding/json"
	"slices"
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

// presenceQueryBatches sends every ID once, in order, each batch's frame
// fits the gateway's limit, and no batch but the last could take the
// next ID.
func TestPresenceQueryBatchesCoverInFittingFrames(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ids := rapid.SliceOfN(proptest.ID("U"), 0, 1200).Draw(t, "ids")
		batches := presenceQueryBatches(ids)
		var all []string
		for i, b := range batches {
			frame, _ := json.Marshal(presenceQuery(b))
			if len(frame) > maxClientFrame {
				t.Fatalf("batch %d is %d bytes", i, len(frame))
			}
			if len(b) == 0 {
				t.Fatalf("batch %d is empty", i)
			}
			if i < len(batches)-1 {
				next, _ := json.Marshal(presenceQuery(append(slices.Clone(b), batches[i+1][0])))
				if len(next) <= maxClientFrame {
					t.Fatalf("batch %d could take one more id", i)
				}
			}
			all = append(all, b...)
		}
		if !slices.Equal(all, ids) {
			t.Fatalf("batches cover %d ids, want %d in order", len(all), len(ids))
		}
	})
}
