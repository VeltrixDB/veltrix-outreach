// veltrix-outreach — weekly Slack digest of outreach leads and intent signals.
//
// Part 1 — Outreach leads: GitHub contributors to competing KV databases,
//   scored by relevance, with pre-filled LinkedIn search links and pitch angles.
//   25 new leads per week, 60-day cooldown per person.
//
// Part 2 — Intent signals: Posts and comments on LinkedIn (via Bing Search),
//   Reddit, and Hacker News where someone is actively expressing KV database
//   pain or asking for alternatives — highest-value, real-time buyer signals.
//
// Usage:
//
//	SLACK_OUTREACH_WEBHOOK=https://hooks.slack.com/... veltrix-outreach
//
// Dry run (print to stdout, no webhook):
//
//	OUTREACH_DRY_RUN=1 veltrix-outreach
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	maxLeadsPerWeek = 25
	maxPerRepo      = 50 // with GITHUB_TOKEN
	maxPerRepoNoTok = 10 // without token (stay under 60 req/hr)
	seenPath        = "seen.json"
	lookbackDays    = 7
)

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	webhook := os.Getenv("SLACK_OUTREACH_WEBHOOK")
	dryRun := os.Getenv("OUTREACH_DRY_RUN") == "1"

	if !dryRun && webhook == "" {
		log.Fatal("set SLACK_OUTREACH_WEBHOOK (or OUTREACH_DRY_RUN=1 to print to stdout)")
	}

	since := time.Now().AddDate(0, 0, -lookbackDays)

	// ── Part 1: GitHub contributor leads ─────────────────────────────────────

	seen, _ := loadSeen(seenPath)

	limit := maxPerRepo
	if token == "" {
		limit = maxPerRepoNoTok
		fmt.Fprintln(os.Stderr, "[warn] GITHUB_TOKEN not set — limited to 10 contributors per repo")
	}

	gh := NewGitHub(token)
	allLeads := gh.FetchLeads(limit)
	fmt.Printf("[github] %d unique contributors fetched\n", len(allLeads))

	var fresh []Lead
	for _, l := range allLeads {
		if !seen.Has(l.Handle) {
			fresh = append(fresh, l)
		}
	}
	fmt.Printf("[filter] %d new leads after 60-day cooldown\n", len(fresh))

	for i := range fresh {
		fresh[i].Score = scoreLead(fresh[i])
	}
	sort.Slice(fresh, func(i, j int) bool {
		return fresh[i].Score > fresh[j].Score
	})
	if len(fresh) > maxLeadsPerWeek {
		fresh = fresh[:maxLeadsPerWeek]
	}

	// ── Part 2: Intent signals ────────────────────────────────────────────────

	intentFetchers := []IntentFetcher{
		NewLinkedInBing(),
		NewRedditIntent(),
		NewHNIntent(),
	}

	var mu sync.Mutex
	var allSignals []IntentSignal
	var wg sync.WaitGroup

	for _, f := range intentFetchers {
		wg.Add(1)
		go func(f IntentFetcher) {
			defer wg.Done()
			sigs, err := f.Fetch(since)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] error: %v\n", f.Name(), err)
				return
			}
			fmt.Printf("[%s] %d intent signals\n", f.Name(), len(sigs))
			mu.Lock()
			allSignals = append(allSignals, sigs...)
			mu.Unlock()
		}(f)
	}
	wg.Wait()

	// Score and deduplicate intent signals by URL.
	seen2 := make(map[string]bool)
	var dedupedSignals []IntentSignal
	for _, s := range allSignals {
		if seen2[s.PostURL] {
			continue
		}
		seen2[s.PostURL] = true
		s.Score = scoreIntentSignal(s)
		dedupedSignals = append(dedupedSignals, s)
	}
	sort.Slice(dedupedSignals, func(i, j int) bool {
		return dedupedSignals[i].Score > dedupedSignals[j].Score
	})
	// Cap to top 15 intent signals (3 sources × 5 best each).
	const maxSignals = 15
	if len(dedupedSignals) > maxSignals {
		dedupedSignals = dedupedSignals[:maxSignals]
	}

	// ── Output ────────────────────────────────────────────────────────────────

	if len(fresh) == 0 && len(dedupedSignals) == 0 {
		fmt.Println("Nothing to post this week.")
		return
	}

	if dryRun {
		printLeads(fresh)
		printSignals(dedupedSignals)
	} else {
		if err := postSlack(webhook, fresh, dedupedSignals); err != nil {
			log.Fatalf("Slack error: %v", err)
		}
		fmt.Println("Slack: posted OK")
	}

	// Persist lead seen-state (intent signals are ephemeral, no dedup needed).
	for _, l := range fresh {
		seen.Mark(l.Handle)
	}
	if err := seen.Save(seenPath); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] could not save seen.json: %v\n", err)
	}
}
