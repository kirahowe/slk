package slackclient

import (
	"maps"
	"slices"
	"testing"

	"pgregory.net/rapid"
)

// The Activity total is additive over categories: splitting an
// activity_v2 map into two parts and summing each gives the total of
// the whole, and no categories sum to zero.
func TestSumActivityV2Additive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := rapid.MapOf(rapid.StringMatching(`[a-z_]{1,12}`), rapid.IntRange(0, 1000)).Draw(t, "activity_v2")
		left, right := map[string]int{}, map[string]int{}
		// Sorted, so a seed replays the same draws.
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if rapid.Bool().Draw(t, "left:"+k) {
				left[k] = m[k]
			} else {
				right[k] = m[k]
			}
		}
		if got, want := sumActivityV2(m), sumActivityV2(left)+sumActivityV2(right); got != want {
			t.Fatalf("sum(%v) = %d, sum of parts %v + %v = %d", m, got, left, right, want)
		}
		if got := sumActivityV2(nil); got != 0 {
			t.Fatalf("sum(nil) = %d", got)
		}
	})
}
