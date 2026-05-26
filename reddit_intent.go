package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RedditIntent finds posts and comments where someone is actively looking
// for a KV database solution — the "I need X, what should I use?" signal.
type RedditIntent struct {
	client *http.Client
}

func NewRedditIntent() *RedditIntent {
	return &RedditIntent{client: &http.Client{Timeout: 15 * time.Second}}
}

func (r *RedditIntent) Name() string { return "Reddit (intent)" }

// intentQueries are phrased to catch people who are actively asking
// for help or expressing pain — not generic discussions.
var intentQueries = []struct {
	Query     string
	Subreddit string // empty = all of Reddit
}{
	{"redis alternative", "devops"},
	{"replace redis", ""},
	{"redis memory cost", ""},
	{"rocksdb write amplification", ""},
	{"key value database recommendation", ""},
	{"kv store alternative", ""},
	{"database recommendation key value", "sysadmin"},
	{"redis too expensive", ""},
	{"distributed key value store", "selfhosted"},
	{"ditching redis", ""},
	{"redis scaling cost", "kubernetes"},
	{"storage engine performance", "golang"},
}

type redditIntentResp struct {
	Data struct {
		Children []struct {
			Data struct {
				Title     string  `json:"title"`
				Selftext  string  `json:"selftext"`
				URL       string  `json:"url"`
				Permalink string  `json:"permalink"`
				Author    string  `json:"author"`
				Score     int     `json:"score"`
				CreatedAt float64 `json:"created_utc"`
				Subreddit string  `json:"subreddit"`
				NumComments int   `json:"num_comments"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func (r *RedditIntent) Fetch(since time.Time) ([]IntentSignal, error) {
	seen := make(map[string]bool)
	var signals []IntentSignal

	for i, q := range intentQueries {
		if i > 0 {
			time.Sleep(700 * time.Millisecond) // Reddit rate limit
		}

		base := "https://www.reddit.com"
		if q.Subreddit != "" {
			base += "/r/" + q.Subreddit
		}
		base += "/search.json"

		params := url.Values{
			"q":        {q.Query},
			"sort":     {"new"},
			"limit":    {"10"},
			"t":        {"week"},
			"restrict_sr": {"0"},
		}
		if q.Subreddit != "" {
			params.Set("restrict_sr", "1")
		}

		req, err := http.NewRequest("GET", base+"?"+params.Encode(), nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "veltrix-outreach/1.0 (marketing bot; github.com/VeltrixDB/veltrix-outreach)")

		resp, err := r.client.Do(req)
		if err != nil {
			continue
		}

		var result redditIntentResp
		decodeErr := func() error {
			defer resp.Body.Close()
			return json.NewDecoder(resp.Body).Decode(&result)
		}()
		if decodeErr != nil {
			continue
		}

		for _, child := range result.Data.Children {
			d := child.Data
			pub := time.Unix(int64(d.CreatedAt), 0)
			if pub.Before(since) {
				continue
			}
			link := "https://reddit.com" + d.Permalink
			if seen[link] {
				continue
			}
			seen[link] = true

			// Skip low-signal posts (too few upvotes on old posts).
			if d.Score < 1 {
				continue
			}

			snippet := d.Selftext
			if len(snippet) > 280 {
				snippet = snippet[:277] + "…"
			}
			if snippet == "" {
				snippet = d.Title
			}

			signals = append(signals, IntentSignal{
				Source:     "reddit",
				AuthorName: "u/" + d.Author,
				AuthorURL:  "https://reddit.com/u/" + d.Author,
				PostURL:    link,
				PostTitle:  d.Title,
				Snippet:    truncate(strings.ReplaceAll(snippet, "\n", " "), 280),
				Score:      d.Score + d.NumComments,
				PostedAt:   pub,
			})
		}
	}
	return signals, nil
}

// isIntentPost checks whether a Reddit title expresses active buyer intent.
// Filters out generic news articles and announcements.
func isIntentPost(title string) bool {
	lower := strings.ToLower(title)
	intentPhrases := []string{
		"alternative", "replace", "recommendation", "suggest",
		"looking for", "need help", "best option", "which database",
		"too expensive", "scaling issue", "performance problem",
		"what do you use", "anyone tried", "experience with",
		"migrating from", "ditching", "moving away from",
	}
	for _, phrase := range intentPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// scoreIntentSignal ranks a signal by how actionable it is.
func scoreIntentSignal(s IntentSignal) int {
	score := s.Score // base: engagement (upvotes + comments)

	lower := strings.ToLower(s.PostTitle + " " + s.Snippet)

	// High-intent phrases
	highIntent := []string{
		"redis alternative", "replace redis", "ditching redis",
		"write amplification", "memory cost", "too expensive",
		"looking for", "recommendation", "migrate", "switching from",
	}
	for _, phrase := range highIntent {
		if strings.Contains(lower, phrase) {
			score += 20
		}
	}

	// Pain-point mentions
	painPoints := []string{
		"$", "cost", "expensive", "scale", "latency", "p99",
		"compaction", "memory", "ram", "nvme", "ssd",
	}
	for _, p := range painPoints {
		if strings.Contains(lower, p) {
			score += 5
		}
	}

	return score
}

// redditAuthorProfileURL returns a Reddit user profile URL.
func redditAuthorProfileURL(author string) string {
	return fmt.Sprintf("https://reddit.com/u/%s", author)
}
