// Package version provides formatting helpers for slk's build-time
// version metadata. The build vars themselves live in package main
// (injected via -ldflags by GoReleaser); callers pass the version
// string in so this package stays free of build-time coupling.
package version

// Line is the build identity `slk --version` prints, e.g.
// "slk v1.2.3 (commit abc1234, built 2026-01-02T03:04:05Z)".
func Line(version, commit, date string) string {
	return "slk " + version + " (commit " + commit + ", built " + date + ")"
}

// Short is the build identity :version shows in the status bar, which
// has room for one short toast: the version and the commit cut to 7
// characters, e.g. "slk v1.2.3 (commit abc1234)", or "slk v1.2.3" when
// the build stamped no commit ("" or the "none" default).
func Short(version, commit string) string {
	if commit == "" || commit == "none" {
		return "slk " + version
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return "slk " + version + " (commit " + commit + ")"
}

// osc8 wraps url as a clickable OSC-8 hyperlink whose visible label is
// the URL itself. Terminals without OSC-8 support render the label as
// plain text.
func osc8(url string) string {
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}

// ModalFooter returns the attribution line shown at the bottom of the
// TUI help modal, e.g.:
//
//	slk dev - Made with ❤️ by Kira Howe (https://kirahowe.com)
//
// The URL is OSC-8 wrapped so supporting terminals make it clickable.
func ModalFooter(version string) string {
	return "slk " + version + " - Made with \u2764\ufe0f by Kira Howe (" + osc8("https://kirahowe.com") + ")"
}

// UpstreamCredit is the fainter line under ModalFooter that credits slk's
// original author, who wrote it up to v0.26 (this fork began after
// v0.26.2).
func UpstreamCredit() string {
	return "Built on slk by Grant Ammons (" + osc8("https://grant.dev") + "), up to v0.26"
}
