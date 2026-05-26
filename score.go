package main

import (
	"net/url"
	"strings"
)

// scoreLead assigns a relevance score. Higher = better lead.
// Factors: commit count (effort signal), follower count (influence),
// company type (scale signal), bio keywords (role signal).
func scoreLead(l Lead) int {
	score := 0

	// Commit count: more commits = deeper investment in this tech.
	switch {
	case l.Commits >= 200:
		score += 40
	case l.Commits >= 50:
		score += 25
	case l.Commits >= 20:
		score += 15
	case l.Commits >= 5:
		score += 8
	default:
		score += 3
	}

	// Follower count: higher following = more influence in the community.
	switch {
	case l.Followers >= 1000:
		score += 20
	case l.Followers >= 500:
		score += 15
	case l.Followers >= 200:
		score += 10
	case l.Followers >= 50:
		score += 5
	}

	// Company: scale-heavy companies are higher priority.
	company := strings.ToLower(l.Company)
	highScaleCompanies := []string{
		"meta", "google", "microsoft", "apple", "amazon", "aws",
		"cloudflare", "stripe", "shopify", "netflix", "uber", "lyft",
		"airbnb", "twitter", "x corp", "linkedin", "salesforce",
		"databricks", "snowflake", "confluent", "mongodb", "cockroachdb",
		"pingcap", "tigerbeeetle", "yugabyte", "planetscale", "neon",
		"vercel", "supabase", "turso", "fly.io",
	}
	for _, co := range highScaleCompanies {
		if strings.Contains(company, co) {
			score += 15
			break
		}
	}

	// Bio keywords: role signals.
	bio := strings.ToLower(l.Bio)
	roleKeywords := map[string]int{
		"storage engineer":        15,
		"database engineer":       15,
		"infrastructure engineer": 12,
		"distributed systems":     12,
		"platform engineer":       10,
		"site reliability":        8,
		"sre":                     8,
		"backend engineer":        6,
		"software engineer":       4,
		"cto":                     18,
		"vp engineering":          18,
		"staff engineer":          12,
		"principal engineer":      12,
		"distinguished engineer":  15,
		"fellow":                  15,
	}
	for kw, pts := range roleKeywords {
		if strings.Contains(bio, kw) {
			score += pts
			break
		}
	}

	// Extra bio signal: mentions of pain points VeltrixDB solves.
	painKeywords := []string{
		"write amplification", "lsm", "compaction", "nvme", "rocksdb",
		"leveldb", "storage engine", "kv store", "key-value",
	}
	for _, kw := range painKeywords {
		if strings.Contains(bio, kw) {
			score += 8
		}
	}

	// Penalize if no name (harder to search on LinkedIn).
	if l.Name == l.Handle {
		score -= 10
	}

	// Penalize if no company (less context for LinkedIn search).
	if l.Company == "" {
		score -= 5
	}

	return score
}

func linkedInSearchURL(name, company string) string {
	q := name
	if company != "" {
		q += " " + company
	}
	return "https://www.linkedin.com/search/results/people/?keywords=" + url.QueryEscape(q)
}
