package main

import (
	"context"
	"slices"
	"testing"

	"github.com/gammons/slk/internal/ui"
)

// badgeTotals returns the Activity totals sent to the UI, in order.
func (c *captureSender) badgeTotals(t *testing.T) []int {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	var got []int
	for _, m := range c.sent {
		if b, ok := m.(ui.ActivityBadgeUpdatedMsg); ok {
			if b.TeamID != "T1" {
				t.Errorf("badge msg team = %q, want T1", b.TeamID)
			}
			got = append(got, b.Total)
		}
	}
	return got
}

// badgeReconnect is a reconnect pass for T1 over client, its badge
// totals sent to the returned sender.
func badgeReconnect(t *testing.T, client *fakeCounts, badge *activityBadge) (*reconnectSync, *captureSender) {
	t.Helper()
	db := newTestDB(t)
	seedWorkspaceChannels(t, db, 1)
	sender := &captureSender{}
	return &reconnectSync{
		client: client, db: db, workspaceID: "T1", program: sender,
		activeChannel:  func() string { return "" },
		refreshChannel: func(context.Context, string) {},
		badge:          badge,
	}, sender
}

// A badge_counts_updated that lands while the reconnect's client.counts
// is out is newer than the answer and has already reached the UI, so
// the answer's total must not follow it.
func TestReconnect_BadgeEventMidRequestSupersedesAnswer(t *testing.T) {
	badge := &activityBadge{}
	client := &fakeCounts{activity: 4}
	r, sender := badgeReconnect(t, client, badge)
	client.during = func() { badge.Set(9, activityBadgeReport(sender, "T1")) }

	r.refreshUnreadState()

	if got := sender.badgeTotals(t); !slices.Equal(got, []int{9}) {
		t.Fatalf("badge totals sent = %v, want [9] (the event's, not the older answer's 4)", got)
	}
}

// An event before the request does not hold the answer back: the mark
// is taken when the request starts.
func TestReconnect_BadgeAnswerFollowsEarlierEvent(t *testing.T) {
	badge := &activityBadge{}
	r, sender := badgeReconnect(t, &fakeCounts{activity: 4}, badge)
	badge.Set(9, activityBadgeReport(sender, "T1"))

	r.refreshUnreadState()

	if got := sender.badgeTotals(t); !slices.Equal(got, []int{9, 4}) {
		t.Fatalf("badge totals sent = %v, want [9 4]", got)
	}
}

// A zero total is an answer, not a missing one: it clears the badge.
func TestReconnect_BadgeZeroTotalIsSent(t *testing.T) {
	r, sender := badgeReconnect(t, &fakeCounts{}, &activityBadge{})

	r.refreshUnreadState()

	if got := sender.badgeTotals(t); !slices.Equal(got, []int{0}) {
		t.Fatalf("badge totals sent = %v, want [0]", got)
	}
}

// Of two fetches out at once, the one that answers last is dropped if
// the other was applied in between: whichever began first, its answer
// is no newer than one already shown.
func TestActivityBadge_FetchAnsweringAfterAnotherIsDropped(t *testing.T) {
	var b activityBadge
	var got []int
	report := func(n int) { got = append(got, n) }

	first, second := b.mark(), b.mark()
	if !b.applyFetched(second, 2, report) {
		t.Fatal("the first answer to land was dropped")
	}
	if b.applyFetched(first, 1, report) {
		t.Fatal("an answer landing after another was applied")
	}
	if !slices.Equal(got, []int{2}) {
		t.Fatalf("reported %v, want [2]", got)
	}
}
