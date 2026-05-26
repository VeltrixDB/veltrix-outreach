package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// targetRepos are the competing / adjacent KV database repos.
// Contributors here are the highest-signal leads for VeltrixDB outreach.
var targetRepos = []struct {
	Owner, Repo string
	Label       string
	Pitch       string
}{
	{
		"redis", "redis", "Redis",
		"Redis's full in-memory model costs 15× more than NVMe at scale. VeltrixDB gives you the same P50 on hot keys with cold data spilling to NVMe automatically.",
	},
	{
		"facebook", "rocksdb", "RocksDB",
		"RocksDB's LSM compaction rewrites data 10–30× over its lifetime. VeltrixDB uses WiscKey KV-separation for ~1.0× write amplification — your SSDs last 10× longer.",
	},
	{
		"dgraph-io", "badger", "BadgerDB",
		"BadgerDB proves the Go community wants a native KV store. VeltrixDB takes the same WiscKey approach further: 1024-shard parallel I/O, Raft replication, Kubernetes Operator.",
	},
	{
		"etcd-io", "etcd", "etcd",
		"etcd powers Kubernetes control planes. VeltrixDB handles the data plane: 7.2M reads/s at P99 510 µs with the same Raft foundation.",
	},
	{
		"tikv", "tikv", "TiKV",
		"You understand distributed KV better than most. VeltrixDB is the NVMe-native complement: values written once, never rewritten, ~160 GB for 1B × 128B keys.",
	},
	{
		"apple", "foundationdb", "FoundationDB",
		"FoundationDB contributors understand serious distributed systems. VeltrixDB brings the same production-grade thinking to NVMe-native storage.",
	},
	{
		"dragonflydb", "dragonfly", "Dragonfly",
		"You're already rethinking Redis. VeltrixDB pushes further: values on NVMe instead of RAM, 10× storage cost reduction, write amplification at 1.0×.",
	},
	{
		"google", "leveldb", "LevelDB",
		"LevelDB defined the LSM-tree model. WiscKey (VeltrixDB's foundation) was the FAST '16 paper that fixed LevelDB's write amplification problem.",
	},
	{
		"scylladb", "scylladb", "ScyllaDB",
		"You care about hardware-level performance. VeltrixDB: io_uring SQPOLL reads, C++ ART index on 2MB hugepages, zero-copy CGO bridge. Same DNA.",
	},
	{
		"aerospike", "aerospike-server", "Aerospike",
		"Aerospike is the benchmark for real-time data at NVMe speed. VeltrixDB brings the open-source, Kubernetes-native version of that story.",
	},
	{
		"pingcap", "tidb", "TiDB",
		"TiDB is built on TiKV. You know the distributed KV problem deeply. VeltrixDB is the write-amplification-free layer for pure KV workloads.",
	},
	{
		"cockroachdb", "cockroach", "CockroachDB",
		"CockroachDB engineers understand Raft, replication, and production databases. VeltrixDB is the pure KV version: same Raft, 7.2M reads/s, NVMe-native.",
	},
}

type ghContrib struct {
	Login         string `json:"login"`
	Contributions int    `json:"contributions"`
	Type          string `json:"type"`
}

type ghUser struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	Company     string `json:"company"`
	Bio         string `json:"bio"`
	Location    string `json:"location"`
	HTMLURL     string `json:"html_url"`
	Followers   int    `json:"followers"`
	PublicRepos int    `json:"public_repos"`
}

type GitHub struct {
	token  string
	client *http.Client
}

func NewGitHub(token string) *GitHub {
	return &GitHub{
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GitHub) FetchLeads(maxPerRepo int) []Lead {
	// Deduplicate across repos by GitHub login.
	// If a person contributes to multiple repos, keep the one with more commits.
	best := make(map[string]Lead)

	for _, repo := range targetRepos {
		contribs, err := g.fetchContributors(repo.Owner, repo.Repo, maxPerRepo)
		if err != nil {
			fmt.Printf("[github] %s/%s: %v\n", repo.Owner, repo.Repo, err)
			continue
		}
		fmt.Printf("[github] %s/%s: %d contributors\n", repo.Owner, repo.Repo, len(contribs))

		for i, c := range contribs {
			if i > 0 {
				sleep := 300 * time.Millisecond
				if g.token == "" {
					sleep = 2 * time.Second
				}
				time.Sleep(sleep)
			}

			// Skip bots.
			if c.Type != "User" {
				continue
			}
			lower := strings.ToLower(c.Login)
			if strings.Contains(lower, "bot") || strings.Contains(lower, "dependabot") {
				continue
			}

			user, err := g.fetchUser(c.Login)
			if err != nil {
				continue
			}

			lead := Lead{
				Handle:      user.Login,
				Name:        cleanName(user.Name, user.Login),
				Company:     cleanCompany(user.Company),
				Bio:         truncate(user.Bio, 120),
				Location:    user.Location,
				Followers:   user.Followers,
				RepoLabel:   repo.Label,
				RepoOwner:   repo.Owner,
				RepoName:    repo.Repo,
				Commits:     c.Contributions,
				PitchAngle:  repo.Pitch,
				GitHubURL:   user.HTMLURL,
				LinkedInURL: linkedInSearchURL(cleanName(user.Name, user.Login), cleanCompany(user.Company)),
			}

			// Keep the highest-commit version if seen in multiple repos.
			if prev, ok := best[user.Login]; !ok || c.Contributions > prev.Commits {
				best[user.Login] = lead
			}
		}
	}

	leads := make([]Lead, 0, len(best))
	for _, l := range best {
		leads = append(leads, l)
	}
	return leads
}

func (g *GitHub) fetchContributors(owner, repo string, limit int) ([]ghContrib, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=%d", owner, repo, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	g.setHeaders(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("repo not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var contribs []ghContrib
	return contribs, json.NewDecoder(resp.Body).Decode(&contribs)
}

func (g *GitHub) fetchUser(login string) (ghUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/users/"+login, nil)
	if err != nil {
		return ghUser{}, err
	}
	g.setHeaders(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return ghUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ghUser{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var u ghUser
	return u, json.NewDecoder(resp.Body).Decode(&u)
}

func (g *GitHub) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func cleanName(name, login string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return login
	}
	return name
}

func cleanCompany(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	// Normalize common aliases.
	replacements := map[string]string{
		"fb":               "Meta",
		"facebook":         "Meta",
		"google llc":       "Google",
		"amazon":           "Amazon/AWS",
		"microsoft corp":   "Microsoft",
		"apple inc":        "Apple",
		"cloudflare, inc.": "Cloudflare",
	}
	lower := strings.ToLower(s)
	for k, v := range replacements {
		if lower == k {
			return v
		}
	}
	// Title-case single-word companies.
	if !strings.Contains(s, " ") && len(s) > 0 {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
