package slackclient

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// canvasClient is a Client whose requests go to h in memory, API and
// files.slack.com alike.
func canvasClient(h http.HandlerFunc) *Client {
	return &Client{
		apiBaseURL: "https://slack.test/api/",
		httpClient: handlerClient(h),
		token:      "xoxc-test",
	}
}

// The shape conversations.info returned for a DM with a 1:1 notes
// canvas (2026-10-21), IDs replaced.
const dmInfo = `{"ok":true,"channel":{"id":"D1","is_im":true,"properties":{
	"meeting_notes":{"file_id":"F2"},
	"tabs":[{"id":"Ct1","type":"canvas","data":{"file_id":"F2","shared_ts":"1.0"},"label":""},
	        {"id":"Ct2","type":"list","data":{"file_id":"F9"}},
	        {"id":"Ct3","type":"canvas","data":{"file_id":"F3"}}],
	"canvas":{"file_id":"F1","is_empty":false}}}}`

func TestConversationCanvasesOrdersAndDedupes(t *testing.T) {
	var form url.Values
	c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(dmInfo))
	})
	ids, err := c.ConversationCanvases(context.Background(), "D1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"F1", "F2", "F3"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	if form.Get("channel") != "D1" || form.Get("token") != "xoxc-test" {
		t.Fatalf("form = %v", form)
	}
}

func TestConversationCanvasesNoneAndError(t *testing.T) {
	c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"C1"}}`))
	})
	if ids, err := c.ConversationCanvases(context.Background(), "C1"); err != nil || len(ids) != 0 {
		t.Fatalf("ids, err = %v, %v; want none", ids, err)
	}
	c = canvasClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	})
	if _, err := c.ConversationCanvases(context.Background(), "C1"); err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("err = %v", err)
	}
}

const canvasDownload = "https://files.slack.com/files-pri/T1-F2/download/notes"

func TestCanvasFetchesInfoThenDownload(t *testing.T) {
	var auth string
	c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files.info":
			_ = r.ParseForm()
			if r.PostForm.Get("file") != "F2" {
				t.Errorf("files.info file = %q", r.PostForm.Get("file"))
			}
			_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"F2","title":"Notes","name":"notes",
				"permalink":"https://x.slack.com/docs/T1/F2","updated":1792610107,
				"url_private":"https://files.slack.com/files-pri/T1-F2/notes",
				"url_private_download":"` + canvasDownload + `"}}`))
		case r.URL.String() == canvasDownload:
			auth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<div class="quip-canvas-content"></div>`))
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	})
	f, err := c.Canvas(context.Background(), "F2")
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != "F2" || f.Title != "Notes" || f.Updated != 1792610107 || f.Permalink != "https://x.slack.com/docs/T1/F2" {
		t.Fatalf("file = %+v", f)
	}
	if string(f.HTML) != `<div class="quip-canvas-content"></div>` {
		t.Fatalf("HTML = %q", f.HTML)
	}
	if auth != "Bearer xoxc-test" {
		t.Errorf("Authorization = %q", auth)
	}
}

// The token goes to files.slack.com only, whatever files.info names.
func TestCanvasRefusesDownloadOffFilesHost(t *testing.T) {
	for _, u := range []string{
		"https://attacker.test/files.slack.com/files-pri/T1-F2/x",
		"https://files.slack.com.attacker.test/files-pri/T1-F2/x",
		"http://files.slack.com/files-pri/T1-F2/x",
		"",
	} {
		var downloaded bool
		c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/files.info" {
				_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"F2","url_private_download":"` + u + `"}}`))
				return
			}
			downloaded = true
		})
		if _, err := c.Canvas(context.Background(), "F2"); err == nil {
			t.Errorf("%q: no error", u)
		}
		if downloaded {
			t.Errorf("%q: request sent", u)
		}
	}
}

// A redirect off https://files.slack.com is refused, not followed with
// the token.
func TestCanvasRefusesRedirectOffFilesHost(t *testing.T) {
	for _, target := range []string{
		"https://attacker.test/steal",
		"http://files.slack.com/files-pri/T1-F2/plain",
	} {
		var followed bool
		c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/api/files.info":
				_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"F2","url_private_download":"` + canvasDownload + `"}}`))
			case r.URL.String() == canvasDownload:
				http.Redirect(w, r, target, http.StatusFound)
			default:
				followed = true
			}
		})
		if _, err := c.Canvas(context.Background(), "F2"); err == nil || !strings.Contains(err.Error(), "refusing redirect") {
			t.Errorf("%s: err = %v", target, err)
		}
		if followed {
			t.Errorf("%s: redirect followed", target)
		}
	}
}

func TestCanvasDownloadFailures(t *testing.T) {
	c := canvasClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/files.info" {
			_, _ = w.Write([]byte(`{"ok":false,"error":"file_not_found"}`))
		}
	})
	if _, err := c.Canvas(context.Background(), "F2"); err == nil || !strings.Contains(err.Error(), "file_not_found") {
		t.Fatalf("err = %v", err)
	}
	c = canvasClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/files.info" {
			_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"F2","url_private_download":"` + canvasDownload + `"}}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	if _, err := c.Canvas(context.Background(), "F2"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v", err)
	}
}
