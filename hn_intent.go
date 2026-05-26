package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HNIntent finds Ask HN posts and comments where someone is looking
// for KV database solutions. Hacker News has a dense, senior-engineer
// audience — high-quality leads.
type HNIntent struct {
	client *http.Client
}

func NewHNIntent() *HNIntent {
	return &HNIntent{client: &http.Client{Timeout: 15 * time.Second}}
}

func (h *HNIntent) Name() string { return "Hacker News (intent)" }

// hnIntentQueries target Ask HN posts and high-signal discussions.
var hnIntentQueries = []string{
	"redis alternative",
	"replace redis",
	"redis memory cost",
	"rocksdb write amplification",
	"key value database recommendation",
	"kv store",
	"database cost scaling",
	"storage engine alternative",
	"redis expensive",
	"distributed key value",
}

type hnAlgoliaResp struct {
	Hits []struct {
		ObjectID       string   `json:"objectID"`
		Title          string   `json:"title"`
		URL            string   `json:"url"`
		Author         string   `json:"author"`
		Points         int      `json:"points"`
		NumComments    int      `json:"num_comments"`
		CreatedAt      string   `json:"created_at"`
		Tags           []string `json:"_tags"`
		StoryText      string   `json:"story_text"`
		CommentText    string   `json:"comment_text"`
	} `json:"hits"`
}

func (h *HNIntent) Fetch(since time.Time) ([]IntentSignal, error) {
	seen := make(map[string]bool)
	var signals []IntentSignal

	for i, q := range hnIntentQueries {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}

		params := url.Values{
			"query":       {q},
			"tags":        {"(story,ask_hn,show_hn)"},
			"numericFilters": {fmt.Sprintf("created_at_i>%d", since.Unix())},
			"hitsPerPage": {"10"},
		}
		apiURL := "https://hn.algolia.com/api/v1/search_by_date?" + params.Encode()

		resp, err := h.client.Get(apiURL)
		if err != nil {
			continue
		}

		var result hnAlgoliaResp
		decodeErr := func() error {
			defer resp.Body.Close()
			return json.NewDecoder(resp.Body).Decode(&result)
		}()
		if decodeErr != nil {
			continue
		}

		for _, hit := range result.Hits {
			pub, err := time.Parse(time.RFC3339, hit.CreatedAt)
			if err != nil {
				continue
			}
			if pub.Before(since) {
				continue
			}

			postURL := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
			if seen[postURL] {
				continue
			}
			seen[postURL] = true

			// Prefer story title; fall back to first line of text.
			title := hit.Title
			if title == "" {
				text := hit.StoryText + hit.CommentText
				if len(text) > 80 {
					text = text[:77] + "…"
				}
				title = text
			}

			// Build snippet from story text or comment.
			snippet := strings.TrimSpace(hit.StoryText + hit.CommentText)
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			snippet = truncate(snippet, 280)

			signals = append(signals, IntentSignal{
				Source:     "hn",
				AuthorName: hit.Author,
				AuthorURL:  "https://news.ycombinator.com/user?id=" + hit.Author,
				PostURL:    postURL,
				PostTitle:  title,
				Snippet:    snippet,
				Score:      hit.Points + hit.NumComments,
				PostedAt:   pub,
			})
		}
	}
	return signals, nil
}
