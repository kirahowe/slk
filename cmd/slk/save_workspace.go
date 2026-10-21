package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// saveWorkspaceTheme sets theme in [workspaces.<tomlKey>].
func saveWorkspaceTheme(configPath, tomlKey, teamID, teamName, themeName string) error {
	return saveWorkspaceSetting(configPath, tomlKey, teamID, teamName, "theme", tomlString(themeName))
}

// saveWorkspaceWidth sets sidebar_width in [workspaces.<tomlKey>].
func saveWorkspaceWidth(configPath, tomlKey, teamID, teamName string, width int) error {
	return saveWorkspaceSetting(configPath, tomlKey, teamID, teamName, "sidebar_width", strconv.Itoa(width))
}

// saveWorkspaceVersionTS sets version_ts in [workspaces.<tomlKey>].
func saveWorkspaceVersionTS(configPath, tomlKey, teamID, teamName, versionTS string) error {
	return saveWorkspaceSetting(configPath, tomlKey, teamID, teamName, "version_ts", tomlString(versionTS))
}

// saveWorkspaceSetting rewrites or appends `key = value` in
// [workspaces.<tomlKey>]; value is already TOML-encoded. It is a
// textual line rewrite rather than a TOML re-marshal, so user comments
// and field ordering survive the write. tomlKey is the literal TOML key
// in the config: the slug for slug-keyed blocks, the team ID for legacy
// ones. With no such block it appends a legacy-keyed
// [workspaces.<teamID>] block, headed by a "# <teamName>" comment.
func saveWorkspaceSetting(configPath, tomlKey, teamID, teamName, key, value string) error {
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
	lines := strings.Split(string(data), "\n")
	setting := key + " = " + value

	header := fmt.Sprintf("[workspaces.%s]", tomlKey)
	sectionStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			sectionStart = i
			break
		}
	}

	if sectionStart >= 0 {
		end := len(lines)
		for j := sectionStart + 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if t == "" || strings.HasPrefix(t, "[") {
				end = j
				break
			}
		}
		for j := sectionStart + 1; j < end; j++ {
			t := strings.TrimSpace(lines[j])
			// A dotted key (theme.background) is a different key.
			if strings.HasPrefix(t, key) && strings.Contains(t, "=") &&
				!strings.HasPrefix(t, key+".") {
				lines[j] = setting
				return writeConfigAtomic(configPath, []byte(strings.Join(lines, "\n")))
			}
		}
		lines = append(lines[:sectionStart+1], append([]string{setting}, lines[sectionStart+1:]...)...)
		return writeConfigAtomic(configPath, []byte(strings.Join(lines, "\n")))
	}

	// No existing section: append a legacy-keyed block.
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	safeName := sanitizeComment(teamName)
	if safeName == "" {
		safeName = teamID
	}
	lines = append(lines, "# "+safeName, fmt.Sprintf("[workspaces.%s]", teamID), setting)
	return writeConfigAtomic(configPath, []byte(strings.Join(lines, "\n")))
}
