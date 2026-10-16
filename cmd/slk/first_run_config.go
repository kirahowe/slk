package main

import "github.com/gammons/slk/internal/config"

// configAfterOnboarding runs first-run onboarding and returns the config
// the rest of the run must use. Onboarding writes the new workspace's
// [workspaces.<slug>] block to configPath, so cfg, loaded before it ran,
// is out of date.
func configAfterOnboarding(configPath string, cfg config.Config, onboard func() error) (config.Config, error) {
	if err := onboard(); err != nil {
		return cfg, err
	}
	return config.Load(configPath)
}
