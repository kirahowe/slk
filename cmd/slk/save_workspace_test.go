package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// The first run's bug (PR #1): onboarding writes [workspaces.<slug>],
// and saves made by a run that loaded the config before it named the
// team ID instead, appending a second block for the same team. The
// next launch refused the file ("both reference team_id"). A save finds
// the block in the file, so it lands in the slug block whatever the
// caller loaded.
func TestWorkspaceSavesLandInTheSlugBlockOnboardingWrote(t *testing.T) {
	const teamID, slug = "T04T4TH8W", "acme"
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := appendWorkspaceConfigBlock(path, slug, teamID, "ACME Corp"); err != nil {
		t.Fatal(err)
	}

	if err := saveWorkspaceVersionTS(path, teamID, "ACME Corp", "1785403654"); err != nil {
		t.Fatalf("save version_ts: %v", err)
	}
	if err := saveWorkspaceWidth(path, teamID, "ACME Corp", 35); err != nil {
		t.Fatalf("save width: %v", err)
	}
	if err := saveWorkspaceTheme(path, teamID, "ACME Corp", "nord"); err != nil {
		t.Fatalf("save theme: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("config no longer loads: %v\n%s", err, mustRead(t, path))
	}
	want := config.Workspace{TeamID: teamID, Theme: "nord", SidebarWidth: 35, VersionTS: "1785403654"}
	if len(got.Workspaces) != 1 || !reflect.DeepEqual(got.Workspaces[slug], want) {
		t.Errorf("workspaces = %+v, want only %q: %+v", got.Workspaces, slug, want)
	}
}

// TOML allows blank lines inside a table. A setting below one is
// still in the block, so the save rewrites it rather than adding a
// second line for the same key, which TOML rejects.
func TestWorkspaceSaveSeesPastABlankLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := "[workspaces.acme]\nteam_id = \"T04T4TH8W\"\n\n# remembered\ntheme = \"nord\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveWorkspaceTheme(path, "T04T4TH8W", "Acme", "dracula"); err != nil {
		t.Fatalf("save: %v", err)
	}
	s := string(mustRead(t, path))
	if strings.Count(s, "theme = ") != 1 || !strings.Contains(s, `theme = "dracula"`) {
		t.Errorf("want one theme line, set to dracula:\n%s", s)
	}
}

func TestWorkspaceSaveEditsAQuotedHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := "[workspaces.\"my team\"]\nteam_id = \"T04T4TH8W\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := saveWorkspaceWidth(path, "T04T4TH8W", "Acme", 30); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspaces) != 1 || got.Workspaces["my team"].SidebarWidth != 30 {
		t.Errorf("workspaces = %+v", got.Workspaces)
	}
}

// A block the line rewrite cannot edit (here an inline table) is
// refused rather than given a second block, and a config that already
// fails to load is left alone.
func TestWorkspaceSaveRefusesAndLeavesTheFile(t *testing.T) {
	for name, initial := range map[string]string{
		"inline table":  "workspaces = { acme = { team_id = \"T04T4TH8W\" } }\n",
		"does not load": "[workspaces.a]\nteam_id = \"T04T4TH8W\"\n[workspaces.b]\nteam_id = \"T04T4TH8W\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := saveWorkspaceTheme(path, "T04T4TH8W", "Acme", "nord"); err == nil {
				t.Error("save succeeded; want a refusal")
			}
			if got := string(mustRead(t, path)); got != initial {
				t.Errorf("file changed:\n%s", got)
			}
		})
	}
}

// genWorkspaceConfig draws a config.toml that loads: an [appearance]
// table, then workspaces keyed by slug (bare or quoted) or by team ID,
// each with some settings, blank lines and comments between its lines,
// and sometimes a sections subtable after it.
func genWorkspaceConfig(t *rapid.T) (data string, teamIDs []string) {
	teamIDs = rapid.SliceOfNDistinct(proptest.ID("T"), 0, 4, rapid.ID[string]).Draw(t, "teams")
	var b strings.Builder
	b.WriteString("[appearance]\ntheme = \"dark\"\n")
	settings := []string{`theme = "nord"`, "sidebar_width = 28", `version_ts = "1700000000"`, "order = 2"}
	for i, id := range teamIDs {
		b.WriteString("\n# workspace\n")
		var key string
		switch rapid.IntRange(0, 2).Draw(t, "keying") {
		case 0:
			key = id
			fmt.Fprintf(&b, "[workspaces.%s]\n", key)
		case 1:
			key = "ws" + strconv.Itoa(i)
			fmt.Fprintf(&b, "[workspaces.%s]\nteam_id = %q\n", key, id)
		default:
			key = fmt.Sprintf("%q", fmt.Sprintf("my ws %d", i))
			fmt.Fprintf(&b, "[workspaces.%s]\nteam_id = %q\n", key, id)
		}
		for _, s := range rapid.SliceOfDistinct(rapid.SampledFrom(settings), rapid.ID[string]).Draw(t, "settings") {
			if rapid.Bool().Draw(t, "blank") {
				b.WriteString("\n")
			}
			if rapid.Bool().Draw(t, "comment") {
				b.WriteString("# note\n")
			}
			b.WriteString(s + "\n")
		}
		if rapid.Bool().Draw(t, "sections") {
			fmt.Fprintf(&b, "\n[workspaces.%s.sections.Eng]\nchannels = [\"eng-*\"]\n", key)
		}
	}
	return b.String(), teamIDs
}

// For any config that loads and any team, known or new, setting a
// workspace's theme yields a config that loads, with exactly one block
// for the team whose theme is the new one, and every other workspace
// as it was.
func TestSetWorkspaceSettingProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data, teamIDs := genWorkspaceConfig(t)
		before, err := config.Parse([]byte(data))
		if err != nil {
			t.Fatalf("generator drew a config that does not load: %v\n%s", err, data)
		}
		target := rapid.SampledFrom(append(teamIDs, "T09NEWTEAM")).Draw(t, "target")
		theme := rapid.StringMatching(`[a-z ]{1,12}`).Draw(t, "theme")

		out, err := setWorkspaceSetting([]byte(data), target, "Team", "theme", tomlString(theme))
		if err != nil {
			t.Fatalf("set: %v\n%s", err, data)
		}
		after, err := config.Parse(out)
		if err != nil {
			t.Fatalf("output does not load: %v\n%s", err, out)
		}

		blocks := 0
		for k, w := range after.Workspaces {
			if w.TeamID == target {
				blocks++
				if w.Theme != theme {
					t.Fatalf("%s theme = %q, want %q\n%s", k, w.Theme, theme, out)
				}
				if old, ok := before.Workspaces[k]; ok {
					old.Theme = theme
					if !reflect.DeepEqual(old, w) {
						t.Fatalf("%s changed beyond its theme: %+v -> %+v\n%s", k, before.Workspaces[k], w, out)
					}
				}
				continue
			}
			if !reflect.DeepEqual(before.Workspaces[k], w) {
				t.Fatalf("%s changed: %+v -> %+v\n%s", k, before.Workspaces[k], w, out)
			}
		}
		if blocks != 1 {
			t.Fatalf("%d blocks for %s\n%s", blocks, target, out)
		}
	})
}
