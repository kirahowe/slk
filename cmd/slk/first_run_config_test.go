package main

import (
	"path/filepath"
	"testing"

	"github.com/gammons/slk/internal/config"
)

// Onboarding writes [workspaces.<slug>] after run() has loaded the
// config, so run() goes on with the config configAfterOnboarding
// returns, which has the new block.
func TestConfigAfterOnboardingSeesTheNewWorkspace(t *testing.T) {
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
	if _, ok := cfg.Workspaces[slug]; !ok {
		t.Errorf("config after onboarding has no %q block: %+v", slug, cfg.Workspaces)
	}
}
