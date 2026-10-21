package main

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/gammons/slk/internal/debuglog"
	slackclient "github.com/gammons/slk/internal/slack"
	"github.com/gammons/slk/internal/ui"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// bootstrapPresenceAndDND fetches the user's current presence and DND
// state from Slack, stores it in wctx.Self, and sends a StatusChangeMsg.
// Also subscribes to presence_change events for the self user and every
// DM peer so external state changes arrive over the WS.
// OnConnect runs it through wctx.selfFetch, never two at once.
func bootstrapPresenceAndDND(ctx context.Context, wctx *WorkspaceContext, program teaSender, peerIDs []string) {
	if wctx == nil || wctx.Client == nil {
		return
	}

	// Subscribe to presence for our own user plus every 1:1 DM peer so the
	// sidebar can show who is online. presence_sub REPLACES the prior
	// subscription set, so self and peers must go in one call. Failure is
	// non-fatal — manual_presence_change and dnd_updated work without it.
	//
	// This runs from OnConnect, which fires on the initial connect AND on
	// every reconnect. Re-subscribing per connection is required because
	// the subscription is connection-scoped.
	subscribeWorkspacePresence(wctx, peerIDs)

	// Record the write counts before asking, so a manual_presence_change
	// or dnd_updated that lands while the requests are out is not
	// overwritten by the older answer.
	mark := wctx.Self.mark()

	var presence *string
	if p, err := wctx.Client.GetUserPresence(ctx, wctx.UserID); err == nil && p != nil {
		presence = &p.Presence
	}

	// DNDStateFromStatus distinguishes an active snooze/scheduled window
	// from a merely configured DND schedule.
	var dnd *fetchedDND
	if st, err := wctx.Client.GetDNDInfo(ctx, wctx.UserID); err == nil && st != nil {
		isDND, endUnix := slackclient.DNDStateFromStatus(*st, time.Now().Unix())
		dnd = &fetchedDND{enabled: isDND}
		if endUnix > 0 {
			dnd.end = time.Unix(endUnix, 0)
		}
	}
	wctx.Self.applyFetched(mark, presence, dnd)

	// DM peers' DND, which the sidebar marks. Later changes arrive as
	// dnd_invalidated and go through the same refresher. Runs on every
	// connect because the socket does not replay missed invalidations.
	wctx.PeerStatus.RefreshDND(ctx, peerIDs)

	wctx.Self.Report(func(st selfState) {
		if program != nil {
			program.Send(statusChangeMsg(wctx.TeamID, st))
		}
	})
}

// selfStatusTimeout bounds one connect-time presence and DND fetch, peer
// DND included, so a run a hung request holds up cannot keep the next
// connect's run waiting indefinitely.
const selfStatusTimeout = 30 * time.Second

// statusChangeMsg reports st as teamID's self status.
func statusChangeMsg(teamID string, st selfState) ui.StatusChangeMsg {
	return ui.StatusChangeMsg{
		TeamID:     teamID,
		Presence:   st.Presence,
		DNDEnabled: st.DNDEnabled,
		DNDEndTS:   st.DNDEndTS,
	}
}

// subscribeWorkspacePresence subscribes over the WebSocket to presence
// updates for the authenticated user plus every 1:1 DM peer, so the
// sidebar can show who is online. Slack's presence_sub REPLACES the prior
// subscription set and is connection-scoped, so this sends self + all DM
// peers in a single call and must be re-run on each (re)connect.
//
// Note: the WS is opened with no_query_on_subscribe=1, so Slack does not
// reply with each peer's current presence at subscribe time — DM rows are
// seeded from the local cache at build time and then updated live by
// presence_change events. Safe to call repeatedly.
func subscribeWorkspacePresence(wctx *WorkspaceContext, ids []string) {
	if wctx == nil || wctx.Client == nil {
		return
	}
	if len(ids) == 0 {
		debuglog.General("subscribeWorkspacePresence: no ids to subscribe")
		return
	}
	n, err := wctx.Client.SubscribePresence(ids)
	if err != nil {
		debuglog.General("subscribeWorkspacePresence (%d ids) FAILED: %v", len(ids), err)
		return
	}
	debuglog.General("subscribeWorkspacePresence: sent presence_sub for %d of %d ids (self+%d dm peers)", n, len(ids), len(ids)-1)
}

// workspacePresenceIDs returns the deduped list of user IDs to subscribe
// for presence: the authenticated user plus every 1:1 DM peer. Group DMs
// and app/bot DMs (which carry no human presence dot in the sidebar) are
// skipped. Reads the published snapshots, so any goroutine may call it.
//
// The order is priority, because one presence_sub frame holds only about
// 300 IDs (SubscribePresence keeps the prefix that fits): self, then
// starred DMs, then the rest by most recent visit, ties in sidebar order.
func workspacePresenceIDs(wctx *WorkspaceContext) []string {
	seen := make(map[string]struct{})
	channels := slices.Clone(wctx.Channels())
	visited := make(map[string]int64)
	for _, it := range wctx.FinderItems() {
		visited[it.ID] = it.LastVisited
	}
	slices.SortStableFunc(channels, func(a, b sidebar.ChannelItem) int {
		if a.IsStarred != b.IsStarred {
			if a.IsStarred {
				return -1
			}
			return 1
		}
		return cmp.Compare(visited[b.ID], visited[a.ID])
	})
	ids := make([]string, 0, len(channels)+1)
	add := func(uid string) {
		if uid == "" {
			return
		}
		if _, ok := seen[uid]; ok {
			return
		}
		seen[uid] = struct{}{}
		ids = append(ids, uid)
	}
	add(wctx.UserID) // self — keeps the self presence subscription intact
	for _, ch := range channels {
		if ch.Type == "dm" {
			add(ch.DMUserID)
		}
	}
	return ids
}

func (h *rtmEventHandler) OnPresenceChange(userID, presence string) {
	_ = h.db.UpdatePresence(userID, presence)
	if h.program == nil {
		return
	}
	h.program.Send(ui.PresenceChangeMsg{
		UserID:   userID,
		Presence: presence,
	})
}

func (h *rtmEventHandler) OnSelfPresenceChange(presence string) {
	if h.wsCtx == nil {
		return
	}
	// Slack uses "active"/"away" in events; store verbatim.
	h.wsCtx.Self.SetPresence(presence, h.reportSelfStatus)
}

func (h *rtmEventHandler) OnDNDChange(enabled bool, endUnix int64) {
	if h.wsCtx == nil {
		return
	}
	var end time.Time
	if endUnix > 0 {
		end = time.Unix(endUnix, 0)
	}
	h.wsCtx.Self.SetDND(enabled, end, h.reportSelfStatus)
}

func (h *rtmEventHandler) reportSelfStatus(st selfState) {
	if h.program != nil {
		h.program.Send(statusChangeMsg(h.workspaceID, st))
	}
}
