package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gammons/slk/internal/config"
)

// A first run onboards a workspace (which writes [workspaces.<slug>]) and
// then connects it, which saves version_ts and sidebar_width under the key
// the run's config names for that team. Those saves must land in the slug
// block: a second block keyed by the team ID makes config.Load reject the
// file ("both reference team_id") on the next launch.
func TestFirstRunSavesLandInTheSlugBlock(t *testing.T) {
	const teamID, slug = "T04T4TH8W", "acme"
	path := filepath.Join(t.TempDir(), "config.toml")

	// run() loads the config before onboarding: the file does not exist yet.
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	cfg, err = configAfterOnboarding(path, cfg, func() error {
		return appendWorkspaceConfigBlock(path, slug, teamID, "ACME Corp")
	})
	if err != nil {
		t.Fatalf("onboarding: %v", err)
	}

	// connectWorkspace's version_ts save.
	key := workspaceTOMLKey(cfg, teamID)
	if err := saveWorkspaceVersionTS(path, key, teamID, "ACME Corp", "1785403654"); err != nil {
		t.Fatalf("save version_ts: %v", err)
	}
	// The sidebar width saver.
	store := newWorkspaceConfigStore(cfg)
	key = store.SaveSidebarWidth(teamID, 35)
	if err := saveWorkspaceWidth(path, key, teamID, "ACME Corp", 35); err != nil {
		t.Fatalf("save width: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		data, _ := os.ReadFile(path)
		t.Fatalf("config no longer loads after first run: %v\n%s", err, data)
	}
	ws := got.Workspaces[slug]
	if ws.VersionTS != "1785403654" || ws.SidebarWidth != 35 {
		t.Errorf("saves missed the %q block: %+v", slug, ws)
	}
}
