package slackclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gammons/slk/internal/slackhttp"
)

// ConversationCanvases returns the file IDs of a conversation's canvases
// from conversations.info's properties: the channel canvas first, then
// each canvas tab in tab order (a DM's 1:1 notes canvas is one), then
// meeting_notes, without duplicates. slack-go's Properties has no tab
// data, so the response is decoded here.
func (c *Client) ConversationCanvases(ctx context.Context, channelID string) ([]string, error) {
	raw, err := c.postForm(ctx, "conversations.info", url.Values{"channel": {channelID}})
	if err != nil {
		return nil, err
	}
	var resp struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Channel struct {
			Properties struct {
				Canvas struct {
					FileID string `json:"file_id"`
				} `json:"canvas"`
				Tabs []struct {
					Type string `json:"type"`
					Data struct {
						FileID string `json:"file_id"`
					} `json:"data"`
				} `json:"tabs"`
				MeetingNotes struct {
					FileID string `json:"file_id"`
				} `json:"meeting_notes"`
			} `json:"properties"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing conversations.info: %w (body=%s)", err, truncateForLog(raw))
	}
	if !resp.OK {
		return nil, fmt.Errorf("conversations.info: %s", resp.Error)
	}
	props := resp.Channel.Properties
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		for _, have := range ids {
			if have == id {
				return
			}
		}
		ids = append(ids, id)
	}
	add(props.Canvas.FileID)
	for _, tab := range props.Tabs {
		if tab.Type == "canvas" {
			add(tab.Data.FileID)
		}
	}
	add(props.MeetingNotes.FileID)
	return ids, nil
}

// CanvasFile is a canvas's files.info metadata and its HTML.
type CanvasFile struct {
	ID        string
	Title     string
	Permalink string
	// Updated is the unix time of the last edit.
	Updated int64
	HTML    []byte
}

// Canvas fetches a canvas: files.info for its metadata and download URL,
// then the download, which is the canvas as HTML. Slack has no read API
// for canvases; this is how the web client's export reads one too.
func (c *Client) Canvas(ctx context.Context, fileID string) (CanvasFile, error) {
	raw, err := c.postForm(ctx, "files.info", url.Values{"file": {fileID}})
	if err != nil {
		return CanvasFile{}, err
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		File  struct {
			ID                 string `json:"id"`
			Title              string `json:"title"`
			Name               string `json:"name"`
			Permalink          string `json:"permalink"`
			Updated            int64  `json:"updated"`
			URLPrivate         string `json:"url_private"`
			URLPrivateDownload string `json:"url_private_download"`
		} `json:"file"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return CanvasFile{}, fmt.Errorf("parsing files.info: %w (body=%s)", err, truncateForLog(raw))
	}
	if !resp.OK {
		return CanvasFile{}, fmt.Errorf("files.info: %s", resp.Error)
	}
	f := resp.File
	out := CanvasFile{ID: f.ID, Title: f.Title, Permalink: f.Permalink, Updated: f.Updated}
	if out.Title == "" {
		out.Title = f.Name
	}
	src := f.URLPrivateDownload
	if src == "" {
		src = f.URLPrivate
	}
	// The session's token and cookie go only to files.slack.com, checked
	// by exact host, whatever the response names.
	if slackhttp.TeamIDFromFilesURL(src) == "" {
		return out, fmt.Errorf("canvas %s: download URL is not on files.slack.com", fileID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp2, err := c.apiHTTPClient().Do(req)
	if err != nil {
		return out, fmt.Errorf("downloading canvas %s: %w", fileID, err)
	}
	defer resp2.Body.Close()
	body, err := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		return out, fmt.Errorf("downloading canvas %s: HTTP %s", fileID, resp2.Status)
	}
	if err != nil {
		return out, fmt.Errorf("downloading canvas %s: %w", fileID, err)
	}
	out.HTML = body
	return out, nil
}
