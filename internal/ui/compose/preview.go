package compose

// The draft preview's state (ctrl+r): whether this box shows it, and
// the draft's text as a send would post it. The App draws it
// (internal/ui/draft_preview.go).

// PreviewOn reports whether the draft preview is showing for this box.
func (m *Model) PreviewOn() bool { return m.preview }

// TogglePreview shows or hides the draft preview and returns the new
// state.
func (m *Model) TogglePreview() bool {
	m.SetPreview(!m.preview)
	return m.preview
}

// SetPreview shows or hides the draft preview. Hiding it drops the
// cached text, so a closed preview holds nothing.
func (m *Model) SetPreview(on bool) {
	if !on {
		m.previewText = previewText{}
	}
	if m.preview == on {
		return
	}
	m.preview = on
	m.dirty()
}

// previewText is the draft's wire text for the draft text and mention
// data it was made from.
type previewText struct {
	typed, wire string
	gen         uint64
	ok          bool
}

// PreviewWire is the draft with its mentions translated as Enter
// translates them (TranslateMentionsForSend), the text a send posts.
// It is translated again only when the draft or the mention data has
// changed since the last call.
func (m *Model) PreviewWire() string {
	typed := m.Value()
	if c := m.previewText; c.ok && c.typed == typed && c.gen == m.mentionGen {
		return c.wire
	}
	wire := m.TranslateMentionsForSend(typed)
	m.previewText = previewText{typed: typed, wire: wire, gen: m.mentionGen, ok: true}
	return wire
}
