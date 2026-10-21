package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gammons/slk/internal/config"
)

// saveWorkspaceTheme sets theme in teamID's [workspaces.*] block.
func saveWorkspaceTheme(configPath, teamID, teamName, themeName string) error {
	return saveWorkspaceSetting(configPath, teamID, teamName, "theme", tomlString(themeName))
}

// saveWorkspaceWidth sets sidebar_width in teamID's [workspaces.*] block.
func saveWorkspaceWidth(configPath, teamID, teamName string, width int) error {
	return saveWorkspaceSetting(configPath, teamID, teamName, "sidebar_width", strconv.Itoa(width))
}

// saveWorkspaceVersionTS sets version_ts in teamID's [workspaces.*] block.
func saveWorkspaceVersionTS(configPath, teamID, teamName, versionTS string) error {
	return saveWorkspaceSetting(configPath, teamID, teamName, "version_ts", tomlString(versionTS))
}

// saveWorkspaceSetting sets `key = value` in teamID's [workspaces.*]
// block of the config file; value is already TOML-encoded. See
// setWorkspaceSetting for how the block is found.
func saveWorkspaceSetting(configPath, teamID, teamName, key, value string) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()

	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return err
		}
		data = nil
	} else if err != nil {
		return err
	}
	out, err := setWorkspaceSetting(data, teamID, teamName, key, value)
	if err != nil {
		return fmt.Errorf("saving %s: %w", key, err)
	}
	return writeConfigAtomic(configPath, out)
}

// setWorkspaceSetting returns data with `key = value` set in teamID's
// [workspaces.*] block. It is a textual line rewrite rather than a TOML
// re-marshal, so user comments and field ordering survive the write.
//
// The block is found in data itself, by team_id, never from a config
// loaded earlier: one loaded before onboarding wrote the slug block, or
// before the user renamed it, names no block or the wrong one, and
// appending a second block for the same team makes the file
// unloadable. With no block for teamID it appends a legacy-keyed
// [workspaces.<teamID>] block, headed by a "# <teamName>" comment.
//
// It refuses (returns an error) when data does not load, when the block
// is not a plain [workspaces.<key>] table it can edit, and when its
// output would not load.
func setWorkspaceSetting(data []byte, teamID, teamName, key, value string) ([]byte, error) {
	cfg, err := config.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config does not load: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	setting := key + " = " + value

	tomlKey, ok := "", false
	for k, w := range cfg.Workspaces {
		if w.TeamID == teamID {
			tomlKey, ok = k, true
			break
		}
	}

	if ok {
		start := workspaceHeaderLine(lines, tomlKey)
		if start < 0 {
			return nil, fmt.Errorf("no [workspaces.%s] table to edit", tomlKey)
		}
		end := len(lines)
		for j := start + 1; j < len(lines); j++ {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "[") {
				end = j
				break
			}
		}
		set := false
		for j := start + 1; j < end; j++ {
			if k, _, found := strings.Cut(lines[j], "="); found && strings.TrimSpace(k) == key {
				lines[j] = setting
				set = true
				break
			}
		}
		if !set {
			lines = append(lines[:start+1], append([]string{setting}, lines[start+1:]...)...)
		}
	} else {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		safeName := sanitizeComment(teamName)
		if safeName == "" {
			safeName = teamID
		}
		lines = append(lines, "# "+safeName, fmt.Sprintf("[workspaces.%s]", teamID), setting)
	}

	out := []byte(strings.Join(lines, "\n"))
	if _, err := config.Parse(out); err != nil {
		return nil, fmt.Errorf("the result would not load: %w", err)
	}
	return out, nil
}

// workspaceHeaderLine returns the index of the [workspaces.<tomlKey>]
// header in lines, bare or quoted, or -1.
func workspaceHeaderLine(lines []string, tomlKey string) int {
	bare := "[workspaces." + tomlKey + "]"
	quoted := "[workspaces." + tomlString(tomlKey) + "]"
	for i, line := range lines {
		if t := strings.TrimSpace(line); t == bare || t == quoted {
			return i
		}
	}
	return -1
}
