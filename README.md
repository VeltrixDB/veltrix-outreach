# veltrix-outreach

Weekly Slack digest of outreach leads and real-time intent signals for VeltrixDB.

Every Monday at 9 AM IST, the tool posts two sections to Slack:

**Part 1 — Outreach leads** (25 new people, 60-day cooldown)
GitHub contributors to competing KV databases, ranked by relevance:
- Name, company, location, follower count
- Commit count and which competing database they maintain
- A tailored pitch angle for their specific pain
- GitHub link + pre-filled LinkedIn people-search URL

**Part 2 — Intent signals** (top 15, refreshed weekly)
People actively expressing KV database pain right now on LinkedIn, Reddit, and HN:
- 🔵 LinkedIn posts (via Bing Search) — "looking for a Redis alternative"
- 🟠 Reddit posts — r/devops, r/sysadmin asking for database recommendations
- 🔶 HN Ask posts — "Ask HN: Redis is too expensive at scale"

These are the highest-priority leads — someone who is publicly asking for a Redis alternative is more valuable than a passive GitHub contributor.

---

## Setup

### 1. Fork or create this repo in the VeltrixDB org

```bash
gh repo create VeltrixDB/veltrix-outreach --public --clone
```

### 2. Create a Slack webhook

1. Go to your Slack workspace → Apps → Incoming Webhooks
2. Add a new webhook for the `#outreach` channel (or wherever you want leads posted)
3. Copy the webhook URL

### 3. Set the Slack webhook secret

```bash
gh secret set SLACK_OUTREACH_WEBHOOK \
  --repo VeltrixDB/veltrix-outreach \
  --body "https://hooks.slack.com/services/..."
```

### 4. (Optional) Get a free Bing Search key for LinkedIn monitoring

Without this, LinkedIn signals are skipped. Reddit and HN still run.

1. Go to [portal.azure.com](https://portal.azure.com)
2. Create resource → search "Bing Search v7" → Create (Free tier F1: 1000 queries/month)
3. Copy the key from Keys and Endpoint

```bash
gh secret set BING_SEARCH_KEY \
  --repo VeltrixDB/veltrix-outreach \
  --body "your_bing_key_here"
```

`GITHUB_TOKEN` is auto-provided by GitHub Actions — no manual setup needed.

### 4. Test with a manual trigger

```bash
gh workflow run weekly-outreach.yml --repo VeltrixDB/veltrix-outreach
```

Or dry-run locally:

```bash
go build -o veltrix-outreach .
OUTREACH_DRY_RUN=1 ./veltrix-outreach
```

---

## Scoring

Leads are ranked by a score that weights:

| Factor | Points |
|--------|--------|
| 200+ commits to competing DB | +40 |
| Works at a high-scale company (Meta, Google, Cloudflare, etc.) | +15 |
| CTO / VP Engineering in bio | +18 |
| Staff / Principal / Distinguished engineer | +12 |
| Storage engineer in bio | +15 |
| 1000+ GitHub followers | +20 |
| Bio mentions pain points VeltrixDB solves | +8 each |
| No real name (harder to find on LinkedIn) | −10 |
| No company listed | −5 |

---

## Databases searched

| Repo | Why |
|------|-----|
| `redis/redis` | In-memory at scale — RAM cost pain |
| `facebook/rocksdb` | LSM write amplification pain |
| `dgraph-io/badger` | Go-native KV — same community |
| `etcd-io/etcd` | Distributed KV — familiar territory |
| `tikv/tikv` | Distributed KV at scale |
| `apple/foundationdb` | Enterprise distributed KV |
| `dragonflydb/dragonfly` | Already looking for Redis alternatives |
| `google/leveldb` | LSM origins — WiscKey is the fix |
| `scylladb/scylladb` | Hardware-level performance focus |
| `aerospike/aerospike-server` | NVMe-first mindset |
| `pingcap/tidb` | TiKV users |
| `cockroachdb/cockroach` | Raft + distributed DB engineers |

---

## State

`seen.json` tracks every handle that's been sent. Records expire after 60 days, so a person re-appears if you haven't reached them. The file is committed back to this repo after every run.

---

## Cron schedule

Every Monday at 9:00 AM IST (3:30 AM UTC).

To change: edit `cron:` in `.github/workflows/weekly-outreach.yml`.
