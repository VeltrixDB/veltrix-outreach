package main

// Lead is a potential VeltrixDB outreach target sourced from GitHub.
type Lead struct {
	Handle      string // GitHub login
	Name        string
	Company     string
	Bio         string
	Location    string
	Followers   int
	RepoLabel   string // e.g. "RocksDB"
	RepoOwner   string // e.g. "facebook"
	RepoName    string // e.g. "rocksdb"
	Commits     int
	PitchAngle  string
	GitHubURL   string
	LinkedInURL string // LinkedIn people-search URL, pre-filled
	Score       int
}
