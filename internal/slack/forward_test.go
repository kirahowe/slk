package slackclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

// The capture's own values, so the built form can be compared with it.
const (
	shareSource      = "CFAKE0001"
	shareTS          = "1700000000.000100"
	shareDestination = "CFAKE0002"
)

func TestShareMessageForm_MatchesCapture(t *testing.T) {
	got := shareMessageForm(shareSource, shareTS, shareDestination, newClientMsgID())
	assertFormMatchesCapture(t, got, "chat_share_message.json")
}

// businessFields is a request's form as capturedForm reads a capture: no
// token, no _x_* envelope.
func businessFields(form url.Values) url.Values {
	out := url.Values{}
	for k, v := range form {
		if k == "token" || strings.HasPrefix(k, "_x_") {
			continue
		}
		out[k] = v
	}
	return out
}

func TestForwardMessage(t *testing.T) {
	const permalink = "https://example.invalid/archives/CFAKE0001/p1700000000000100"
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantTS    string
		wantError string
	}{
		{"success", http.StatusOK, `{"ok":true,"channel":"CFAKE0002","ts":"1700000001.000200"}`, "1700000001.000200", ""},
		{"rejected", http.StatusOK, `{"ok":false,"error":"not_in_channel"}`, "", "not_in_channel"},
		{"empty ts", http.StatusOK, `{"ok":true,"channel":"CFAKE0002","ts":""}`, "", "empty message timestamp"},
		{"missing ts", http.StatusOK, `{"ok":true,"channel":"CFAKE0002"}`, "", "empty message timestamp"},
		{"bad json", http.StatusOK, `<html>`, "", "parsing chat.shareMessage"},
		// Not retried: Slack may have posted the share before refusing.
		{"rate limited", http.StatusTooManyRequests, ``, "", "rate limited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []*http.Request
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				requests = append(requests, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := NewClient("xoxc-test", "d-cookie")
			pointClientAtTestServer(t, c, srv)
			permalinks := 0
			c.api = &mockSlackAPI{getPermalinkContextFn: func(context.Context, *slack.PermalinkParameters) (string, error) {
				permalinks++
				return permalink, nil
			}}

			ts, link, err := c.ForwardMessage(context.Background(), shareSource, shareTS, shareDestination)
			if len(requests) != 1 {
				t.Fatalf("requests = %d, want one chat.shareMessage", len(requests))
			}
			r := requests[0]
			if r.URL.Path != "/api/chat.shareMessage" {
				t.Errorf("path = %q, want /api/chat.shareMessage", r.URL.Path)
			}
			assertFormMatchesCapture(t, businessFields(r.PostForm), "chat_share_message.json")
			if got := r.PostForm.Get("_x_reason"); got != "sendShareMessage" {
				t.Errorf("_x_reason = %q, want the capture's sendShareMessage", got)
			}
			if r.PostForm.Get("token") != "xoxc-test" {
				t.Errorf("token not sent")
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				if ts != "" || link != "" {
					t.Errorf("failed result = (%q, %q), want empty", ts, link)
				}
				if permalinks != 0 {
					t.Errorf("permalink fetched after a failed share")
				}
				return
			}
			if err != nil {
				t.Fatalf("ForwardMessage: %v", err)
			}
			if ts != tc.wantTS || link != permalink {
				t.Errorf("result = (%q, %q), want (%q, %q)", ts, link, tc.wantTS, permalink)
			}
		})
	}
}

// The share already happened, so a permalink failure only empties the
// stand-in's text; it must not report the forward as failed (a retry would
// duplicate it).
func TestForwardMessage_PermalinkFailureIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"channel":"CFAKE0002","ts":"1700000001.000200"}`))
	}))
	defer srv.Close()
	c := NewClient("xoxc-test", "d-cookie")
	pointClientAtTestServer(t, c, srv)
	c.api = &mockSlackAPI{getPermalinkContextFn: func(context.Context, *slack.PermalinkParameters) (string, error) {
		return "", errors.New("boom")
	}}
	ts, link, err := c.ForwardMessage(context.Background(), shareSource, shareTS, shareDestination)
	if err != nil || ts != "1700000001.000200" || link != "" {
		t.Errorf("result = (%q, %q, %v), want ts and empty permalink", ts, link, err)
	}
}

// Two client_msg_ids are never the same: each is fresh randomness, not
// derived from the time or a counter the next call could repeat.
func TestNewClientMsgIDIsFresh(t *testing.T) {
	a, b := newClientMsgID(), newClientMsgID()
	if !clientMsgIDRe.MatchString(a) {
		t.Errorf("client_msg_id = %q, want a version 4 UUID", a)
	}
	if a == b {
		t.Errorf("two client_msg_ids are both %q", a)
	}
}
