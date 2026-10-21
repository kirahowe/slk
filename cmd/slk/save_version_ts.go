package main

import (
	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/slackhttp"
)

// seedVersionTS primes env with the build timestamp cached for teamID
// on a previous run, so the very first request of this session carries
// a current _x_version_ts instead of the compiled-in fallback. A nil
// envelope, an unconfigured workspace, or an empty cached value all
// leave the fallback in place (Envelope.SetVersionTS ignores "").
func seedVersionTS(env *slackhttp.Envelope, cfg config.Config, teamID string) {
	if env == nil {
		return
	}
	ws, ok := cfg.WorkspaceByTeamID(teamID)
	if !ok {
		return
	}
	env.SetVersionTS(ws.VersionTS)
}

// workspaceTOMLKey returns the literal TOML key of the [workspaces.*]
// block configured for teamID. Blocks are keyed either by a user-chosen
// slug (with team_id set) or, for legacy configs, by the raw team ID.
// When no block exists we fall back to the team ID, which is the key
// the saveWorkspace* helpers use when they append a new block.
func workspaceTOMLKey(cfg config.Config, teamID string) string {
	for k, w := range cfg.Workspaces {
		if w.TeamID == teamID {
			return k
		}
	}
	return teamID
}
