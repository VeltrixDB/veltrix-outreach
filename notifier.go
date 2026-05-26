package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// postSlack sends the weekly digest as two separate Slack messages to stay
// under Slack's 50-block-per-message hard limit.
// Message 1: intent signals (real-time buyer signals).
// Message 2: outreach leads in pages of 20 (GitHub contributors).
func postSlack(webhookURL string, leads []Lead, signals []IntentSignal) error {
	week := time.Now().Format("Jan 2, 2006")

	if len(signals) > 0 {
		if err := sendMessage(webhookURL, signalsPayload(signals, week)); err != nil {
			return fmt.Errorf("signals message: %w", err)
		}
	}

	// Send leads in pages of 20 to stay well under 50 blocks per message.
	const pageSize = 20
	for i := 0; i < len(leads); i += pageSize {
		end := i + pageSize
		if end > len(leads) {
			end = len(leads)
		}
		page := i/pageSize + 1
		total := (len(leads) + pageSize - 1) / pageSize
		if err := sendMessage(webhookURL, leadsPayload(leads[i:end], week, page, total)); err != nil {
			return fmt.Errorf("leads message page %d: %w", page, err)
		}
	}
	return nil
}

func sendMessage(webhookURL string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Slack returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// signalsPayload builds the intent signals message (max ~18 blocks for 15 signals).
func signalsPayload(signals []IntentSignal, week string) map[string]any {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": "📢  Intent Signals — Week of " + week,
			},
		},
		{
			"type": "context",
			"elements": []map[string]any{{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%d people* actively expressing KV database pain on LinkedIn, Reddit & HN — highest-priority leads.", len(signals)),
			}},
		},
		{"type": "divider"},
	}
	for _, s := range signals {
		blocks = append(blocks, signalBlock(s))
	}
	blocks = append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{{
			"type": "mrkdwn",
			"text": "Sources: LinkedIn (Bing) · Reddit · Hacker News  |  <https://github.com/VeltrixDB/veltrix-outreach|veltrix-outreach>",
		}},
	})
	return map[string]any{"blocks": blocks}
}

// leadsPayload builds one page of outreach leads (max ~23 blocks for 20 leads).
func leadsPayload(leads []Lead, week string, page, totalPages int) map[string]any {
	title := "🎯  Outreach Leads — Week of " + week
	if totalPages > 1 {
		title = fmt.Sprintf("🎯  Outreach Leads — Week of %s (%d/%d)", week, page, totalPages)
	}
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": title},
		},
		{
			"type": "context",
			"elements": []map[string]any{{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%d new leads* from GitHub contributors to competing KV databases  ·  60-day cooldown", len(leads)),
			}},
		},
		{"type": "divider"},
	}
	for _, l := range leads {
		blocks = append(blocks, leadBlock(l))
	}
	blocks = append(blocks, map[string]any{
		"type": "context",
		"elements": []map[string]any{{
			"type": "mrkdwn",
			"text": "Sources: Redis · RocksDB · BadgerDB · etcd · TiKV · FoundationDB · Dragonfly · LevelDB · ScyllaDB · Aerospike · TiDB · CockroachDB",
		}},
	})
	return map[string]any{"blocks": blocks}
}

// ── Intent signal block ───────────────────────────────────────────────────────

func signalBlock(s IntentSignal) map[string]any {
	icon := sourceIcon(s.Source)

	var sb strings.Builder
	sb.WriteString(icon + "  *" + slackEscape(s.PostTitle) + "*\n")

	// Author line
	if s.AuthorURL != "" {
		sb.WriteString(fmt.Sprintf("<%s|%s>", s.AuthorURL, slackEscape(s.AuthorName)))
	} else {
		sb.WriteString(slackEscape(s.AuthorName))
	}
	if s.Score > 0 {
		sb.WriteString(fmt.Sprintf("  ·  ↑%d", s.Score))
	}
	sb.WriteString("  ·  " + humanAge(s.PostedAt) + "\n")

	// Snippet
	if s.Snippet != "" && s.Snippet != s.PostTitle {
		sb.WriteString("\n_\"" + slackEscape(s.Snippet) + "\"_\n")
	}

	// Links
	sb.WriteString(fmt.Sprintf("\n<%s|View post>", s.PostURL))
	if s.LinkedInSearchURL != "" {
		sb.WriteString(fmt.Sprintf("  ·  <%s|🔍 Find on LinkedIn>", s.LinkedInSearchURL))
	}

	return map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": sb.String(),
		},
	}
}

// ── Contributor lead block ────────────────────────────────────────────────────

func leadBlock(l Lead) map[string]any {
	var sb strings.Builder

	name := slackEscape(l.Name)
	if l.Company != "" {
		name += "  ·  " + slackEscape(l.Company)
	}
	sb.WriteString("*" + name + "*")
	if l.Location != "" {
		sb.WriteString("  📍 " + slackEscape(l.Location))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("%d commits to %s", l.Commits, slackEscape(l.RepoLabel)))
	if l.Followers > 0 {
		sb.WriteString(fmt.Sprintf("  ·  %d followers", l.Followers))
	}
	sb.WriteString("\n")

	if l.Bio != "" {
		sb.WriteString("_" + slackEscape(l.Bio) + "_\n")
	}

	sb.WriteString("\n*Pitch:*  " + slackEscape(truncate(l.PitchAngle, 180)) + "\n")

	sb.WriteString(fmt.Sprintf(
		"\n<https://github.com/%s|GitHub @%s>  ·  <%s|🔍 Search on LinkedIn>",
		l.Handle, l.Handle, l.LinkedInURL,
	))

	return map[string]any{
		"type": "section",
		"text": map[string]any{
			"type": "mrkdwn",
			"text": sb.String(),
		},
	}
}

// ── Dry-run printers ──────────────────────────────────────────────────────────

func printSignals(signals []IntentSignal) {
	if len(signals) == 0 {
		fmt.Println("\nNo intent signals found.")
		return
	}
	fmt.Printf("\n📢  Active Intent Signals (%d)\n", len(signals))
	fmt.Println(strings.Repeat("─", 70))
	for _, s := range signals {
		fmt.Printf("\n%s  [%s]  %s\n", sourceIcon(s.Source), strings.ToUpper(s.Source), s.AuthorName)
		fmt.Printf("    %s\n", s.PostTitle)
		if s.Snippet != "" && s.Snippet != s.PostTitle {
			fmt.Printf("    \"%s\"\n", truncate(s.Snippet, 120))
		}
		fmt.Printf("    Score: %d  ·  %s\n", s.Score, humanAge(s.PostedAt))
		fmt.Printf("    Post:     %s\n", s.PostURL)
		if s.LinkedInSearchURL != "" {
			fmt.Printf("    LinkedIn: %s\n", s.LinkedInSearchURL)
		}
	}
	fmt.Println()
}

func printLeads(leads []Lead) {
	if len(leads) == 0 {
		fmt.Println("\nNo new leads this week.")
		return
	}
	fmt.Printf("\n🎯  Outreach Leads (%d)\n", len(leads))
	fmt.Println(strings.Repeat("─", 70))
	for _, l := range leads {
		fmt.Printf("\n👤  %s", l.Name)
		if l.Company != "" {
			fmt.Printf("  ·  %s", l.Company)
		}
		if l.Location != "" {
			fmt.Printf("  ·  %s", l.Location)
		}
		fmt.Printf("\n    %d commits to %s", l.Commits, l.RepoLabel)
		if l.Followers > 0 {
			fmt.Printf("  ·  %d followers", l.Followers)
		}
		if l.Bio != "" {
			fmt.Printf("\n    %s", l.Bio)
		}
		fmt.Printf("\n    Pitch:    %s", truncate(l.PitchAngle, 140))
		fmt.Printf("\n    GitHub:   %s", l.GitHubURL)
		fmt.Printf("\n    LinkedIn: %s", l.LinkedInURL)
		fmt.Printf("\n    Score: %d\n", l.Score)
	}
	fmt.Println()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func sourceIcon(source string) string {
	switch source {
	case "linkedin":
		return "🔵"
	case "reddit":
		return "🟠"
	case "hn":
		return "🔶"
	default:
		return "💬"
	}
}

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func slackEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
