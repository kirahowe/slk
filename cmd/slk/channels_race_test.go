package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/slack-go/slack"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/ui/peerstatus"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// raceWithConversationOpened runs read in its own goroutine, over and
// over, while the test goroutine plays the WebSocket event owner and
// opens many conversations. The race detector orders accesses by
// happens-before, not timing, so a reader that touches the owner's
// slices without synchronization is reported whichever side of the
// appends it lands on. Both sides run a fixed number of iterations
// and the test blocks on the reader's done signal.
func raceWithConversationOpened(t *testing.T, wctx *WorkspaceContext, read func()) {
	t.Helper()
	h := &rtmEventHandler{wsCtx: wctx, program: sendFunc(func(tea.Msg) {})}
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		for range 300 {
			read()
		}
	}()
	<-started
	for i := range 300 {
		ch := slack.Channel{}
		ch.ID = fmt.Sprintf("D%04d", i)
		ch.IsIM = true
		ch.User = fmt.Sprintf("UP%04d", i)
		h.OnConversationOpened(ch)
	}
	<-done
}

func raceWorkspace() *WorkspaceContext {
	w := &WorkspaceContext{
		TeamID: "T1", UserID: "USELF",
		UserNames: newUserNameStore(nil),
	}
	setConversations(w, []sidebar.ChannelItem{{ID: "D1", Name: "U1", Type: "dm", DMUserID: "U1"}})
	return w
}

// The workspace switch runs in a tea.Cmd, off the event owner.
func TestSwitchListsRacesConversationOpened(t *testing.T) {
	wctx := raceWorkspace()
	raceWithConversationOpened(t, wctx, func() {
		switchLists(wctx, map[string]peerstatus.Status{})
	})
}

// RefreshPeerDND runs in a tea.Cmd and reads the DM peers.
func TestPresenceIDsRaceConversationOpened(t *testing.T) {
	wctx := raceWorkspace()
	raceWithConversationOpened(t, wctx, func() { workspacePresenceIDs(wctx) })
}

// Lookup runs on the UI goroutine.
func TestLookupConversationRacesConversationOpened(t *testing.T) {
	wctx := raceWorkspace()
	raceWithConversationOpened(t, wctx, func() {
		lookupConversation(wctx, "D0299")
		lookupConversation(wctx, "no-such-channel")
	})
}

// The rail's unread dots are computed on the UI goroutine.
func TestRailUnreadRacesConversationOpened(t *testing.T) {
	wctx := raceWorkspace()
	unread := []cache.UnreadChannel{{WorkspaceID: "T1", ChannelID: "D0299"}}
	byID := func(string) *WorkspaceContext { return wctx }
	raceWithConversationOpened(t, wctx, func() {
		railUnreadWorkspaces(unread, []string{"T1"}, byID, func(string, string) bool { return false })
	})
}

// setConversations seeds a test workspace the way connect does.
func setConversations(w *WorkspaceContext, items []sidebar.ChannelItem) {
	w.channels = items
	w.publishChannels()
}

// TestOwnerSlicesStayOnTheOwnerPath pins who may name the mutable
// WorkspaceContext slices. Both are written by the WebSocket event
// owner (and by connectWorkspace before the socket starts), so any
// other reader goes through the Channels() and FinderItems() snapshots.
// Same package, so the compiler cannot enforce it; this test does. A
// file that legitimately runs on the owner is added to the list with a
// reason, and an entry that no longer names the fields must be removed.
// Inside an owner file, a go statement or a tea.Cmd literal still runs
// off the owner, so the fields may not appear in either.
func TestOwnerSlicesStayOnTheOwnerPath(t *testing.T) {
	owner := map[string]string{
		"workspace.go":             "declares the fields and publishes the snapshots",
		"connect.go":               "connectWorkspace builds them before the socket starts; OnConnect and the sync run on the owner",
		"conversations.go":         "OnConversationOpened, addConversation and the DM repair run on the owner",
		"membership.go":            "mute refresh runs on the owner",
		"sections.go":              "section refresh runs on the owner",
		"resolved_peers.go":        "OnPendingEvents runs on the owner",
		"starred_conversations.go": "the starred sweep runs on the owner",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if field, ok := ownerSliceRef(n); ok {
				if _, ok := owner[name]; !ok {
					t.Errorf("%s names WorkspaceContext.%s: only the event owner's files may; read Channels() or FinderItems() instead", name, field)
				}
				used[name] = true
			}
			// Even in an owner file, a goroutine or a tea.Cmd runs off
			// the owner, so neither may reach the slices.
			if body, what := offOwnerBody(n); body != nil {
				ast.Inspect(body, func(n ast.Node) bool {
					if field, ok := ownerSliceRef(n); ok {
						t.Errorf("%s names WorkspaceContext.%s inside %s, which runs off the event owner; read Channels() or FinderItems() instead", name, field, what)
					}
					return true
				})
			}
			return true
		})
	}
	for name := range owner {
		if !used[name] {
			t.Errorf("%s is listed as an owner file but no longer names the slices; remove it", name)
		}
	}
}

// ownerSliceRef reports whether n selects one of the owner's slices.
func ownerSliceRef(n ast.Node) (string, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "channels" && sel.Sel.Name != "finderItems") {
		return "", false
	}
	return sel.Sel.Name, true
}

// offOwnerBody returns the code n starts off the event owner: the call
// of a go statement, or a func literal returning tea.Msg (a tea.Cmd,
// which bubbletea runs on its own goroutine).
func offOwnerBody(n ast.Node) (ast.Node, string) {
	switch n := n.(type) {
	case *ast.GoStmt:
		return n.Call, "a go statement"
	case *ast.FuncLit:
		res := n.Type.Results
		if res == nil || len(res.List) != 1 {
			return nil, ""
		}
		if sel, ok := res.List[0].Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Msg" {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "tea" {
				return n.Body, "a tea.Cmd"
			}
		}
	}
	return nil, ""
}
