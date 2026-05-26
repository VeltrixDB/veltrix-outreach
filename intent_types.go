package main

import "time"

// IntentSignal is a public post or comment where someone is actively expressing
// pain that VeltrixDB solves — high-intent, real-time signals.
type IntentSignal struct {
	Source     string    // "linkedin", "reddit", "hn"
	AuthorName string    // display name or username
	AuthorURL  string    // profile link if extractable
	PostURL    string    // direct link to the post/comment
	PostTitle  string    // thread title or first line of post
	Snippet    string    // relevant excerpt
	Score      int       // upvotes / reactions / HN points
	PostedAt   time.Time
	// LinkedInSearchURL is pre-filled only when a real name is extractable.
	LinkedInSearchURL string
}

// IntentFetcher fetches intent signals from a single source.
type IntentFetcher interface {
	Name() string
	Fetch(since time.Time) ([]IntentSignal, error)
}
