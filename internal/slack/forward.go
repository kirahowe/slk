package slackclient

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gammons/slk/internal/debuglog"
	"github.com/gammons/slk/internal/slackhttp"
)

// ForwardMessage shares a message into another conversation the way the web
// client's Forward does: chat.shareMessage (undocumented), so the recipient
// gets a real shared message rather than a pasted link. It returns the
// destination message's timestamp and, best effort, the source permalink the
// UI shows in its stand-in row until the share's echo arrives (empty when it
// could not be fetched; the share has already happened by then, so that is
// not an error). The share is never retried, a 429 included: a retry after
// Slack accepted it would post the message twice.
func (c *Client) ForwardMessage(ctx context.Context, sourceChannelID, ts, destinationChannelID string) (postedTS string, permalink string, err error) {
	// The _x_reason the web client sends on chat.shareMessage.
	shareCtx := slackhttp.WithReason(ctx, "sendShareMessage")
	raw, err := c.postForm(shareCtx, "chat.shareMessage", shareMessageForm(sourceChannelID, ts, destinationChannelID, newClientMsgID()))
	if err != nil {
		return "", "", fmt.Errorf("forwarding message: %w", err)
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", fmt.Errorf("forwarding message: parsing chat.shareMessage: %w (body=%s)", err, truncateForLog(raw))
	}
	if !resp.OK {
		return "", "", fmt.Errorf("forwarding message: chat.shareMessage: %s", resp.Error)
	}
	if resp.TS == "" {
		return "", "", fmt.Errorf("forwarding message: chat.shareMessage: empty message timestamp")
	}
	permalink, perr := c.GetPermalink(ctx, sourceChannelID, ts)
	if perr != nil {
		debuglog.General("chat.shareMessage: permalink for local echo unavailable: %v", perr)
		permalink = ""
	}
	return resp.TS, permalink, nil
}

// shareMessageForm builds chat.shareMessage's business fields (postForm adds
// the token, slackhttp the _x_* envelope), as the web client sends them with
// no comment typed. Capture: testdata/chat_share_message.json.
func shareMessageForm(sourceChannelID, ts, destinationChannelID, clientMsgID string) url.Values {
	return url.Values{
		"timestamp":             {ts},
		"channel":               {sourceChannelID},
		"share_channel":         {destinationChannelID},
		"client_msg_id":         {clientMsgID},
		"text":                  {""},
		"pending_slug_urls":     {"[]"},
		"skip_dlp_user_warning": {"false"},
	}
}

// newClientMsgID is the web client's client_msg_id: a random version 4
// UUID. The only generator of it in slk.
func newClientMsgID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
