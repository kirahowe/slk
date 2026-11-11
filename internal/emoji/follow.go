package emoji

// FollowSelection returns where a picker's highlight goes when its list
// changes from before to after without the user typing (a new history,
// new custom emoji): the index in after of the entry with the name
// before[selected] has, so the highlight stays on the emoji Enter or
// Tab would have committed. When that name is no longer listed,
// selected is clamped into after; an empty after gives 0. name reads
// an entry's emoji name; a picker lists each name once (see Match).
func FollowSelection[E any](before []E, selected int, after []E, name func(E) string) int {
	if selected >= 0 && selected < len(before) {
		want := name(before[selected])
		for i, e := range after {
			if name(e) == want {
				return i
			}
		}
	}
	return min(max(selected, 0), max(len(after)-1, 0))
}
