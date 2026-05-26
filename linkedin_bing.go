package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// LinkedInBing finds LinkedIn posts via the Bing Web Search API.
// Requires BING_SEARCH_KEY (free Azure tier: 1000 queries/month).
// Get a free key at: portal.azure.com → Create resource → Bing Search v7.
type LinkedInBing struct {
	key    string
	client *http.Client
}

func NewLinkedInBing() *LinkedInBing {
	return &LinkedInBing{
		key:    os.Getenv("BING_SEARCH_KEY"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (l *LinkedInBing) Name() string { return "LinkedIn (Bing)" }

// linkedInSearchQueries are crafted to surface posts where someone is
// actively expressing database pain or asking for alternatives.
var linkedInSearchQueries = []string{
	`site:linkedin.com "redis" ("too expensive" OR "memory costs" OR "running out of memory" OR "looking for alternative")`,
	`site:linkedin.com "rocksdb" ("write amplification" OR "compaction" OR "performance issue")`,
	`site:linkedin.com "key-value database" ("recommendation" OR "suggestions" OR "what do you use" OR "alternatives")`,
	`site:linkedin.com "replace redis" OR "redis replacement" OR "ditching redis"`,
	`site:linkedin.com "database costs" ("scale" OR "infrastructure" OR "storage") "kv" OR "key-value" OR "cache"`,
}

type bingResp struct {
	WebPages struct {
		Value []bingPage `json:"value"`
	} `json:"webPages"`
}

type bingPage struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Snippet         string `json:"snippet"`
	DateLastCrawled string `json:"dateLastCrawled"`
}

func (l *LinkedInBing) Fetch(since time.Time) ([]IntentSignal, error) {
	if l.key == "" {
		return nil, nil // silently skip if no key configured
	}

	seen := make(map[string]bool)
	var signals []IntentSignal

	for i, q := range linkedInSearchQueries {
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		pages, err := l.search(q, 20)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[linkedin-bing] query %d: %v\n", i+1, err)
			continue
		}

		for _, page := range pages {
			if !strings.Contains(page.URL, "linkedin.com") {
				continue
			}
			// Skip job postings and company pages.
			if strings.Contains(page.URL, "/jobs/") ||
				strings.Contains(page.URL, "/company/") {
				continue
			}
			if seen[page.URL] {
				continue
			}
			seen[page.URL] = true

			pub := since // default if parse fails
			if t, err := time.Parse(time.RFC3339, page.DateLastCrawled); err == nil {
				if t.Before(since) {
					continue
				}
				pub = t
			}

			name := extractLinkedInName(page.Name)
			sig := IntentSignal{
				Source:     "linkedin",
				AuthorName: name,
				AuthorURL:  extractLinkedInProfileURL(page.URL),
				PostURL:    page.URL,
				PostTitle:  page.Name,
				Snippet:    truncate(page.Snippet, 280),
				PostedAt:   pub,
			}
			if name != "" {
				sig.LinkedInSearchURL = linkedInSearchURL(name, "")
			}
			signals = append(signals, sig)
		}
	}
	return signals, nil
}

func (l *LinkedInBing) search(query string, count int) ([]bingPage, error) {
	params := url.Values{
		"q":          {query},
		"count":      {fmt.Sprintf("%d", count)},
		"freshness":  {"Week"},
		"mkt":        {"en-US"},
		"textFormat": {"Raw"},
	}
	req, err := http.NewRequest("GET",
		"https://api.bing.microsoft.com/v7.0/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", l.key)

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid Bing API key")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Bing HTTP %d", resp.StatusCode)
	}

	var result bingResp
	return result.WebPages.Value, json.NewDecoder(resp.Body).Decode(&result)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractLinkedInName parses the Bing page title for the person's name.
// Common formats:
//   "John Smith - Principal Engineer at Stripe | LinkedIn"
//   "John Smith | LinkedIn"
//   "Post by John Smith | LinkedIn"
func extractLinkedInName(title string) string {
	title = strings.TrimSuffix(title, " | LinkedIn")
	title = strings.TrimSuffix(title, " - LinkedIn")
	title = strings.TrimPrefix(title, "Post by ")
	title = strings.TrimPrefix(title, "Article by ")
	// Remove the " - Role at Company" suffix.
	if idx := strings.Index(title, " - "); idx > 0 {
		title = title[:idx]
	}
	return strings.TrimSpace(title)
}

// extractLinkedInProfileURL converts a post URL to the author's profile URL.
// Post URL: https://www.linkedin.com/posts/john-smith-xyz_slug-activity-id/
// Profile:  https://www.linkedin.com/in/john-smith-xyz
func extractLinkedInProfileURL(postURL string) string {
	parts := strings.Split(postURL, "/")
	for i, part := range parts {
		if part == "posts" && i+1 < len(parts) {
			slug := parts[i+1]
			// Remove the _slug-activity-id suffix — everything after the last underscore
			// that is followed by the post text.
			if idx := strings.LastIndex(slug, "_"); idx > 0 {
				// Only trim if what follows looks like an activity (contains "activity")
				rest := slug[idx+1:]
				if strings.Contains(rest, "activity") {
					slug = slug[:idx]
				}
			}
			// Remove query/fragment.
			if idx := strings.IndexAny(slug, "?#"); idx >= 0 {
				slug = slug[:idx]
			}
			if slug != "" {
				return "https://www.linkedin.com/in/" + slug
			}
		}
	}
	return postURL
}
