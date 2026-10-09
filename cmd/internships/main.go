package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Job titles we consider relevant. Case-insensitive substring match.
var devopsKeywords = []string{
	"devops",
	"dev ops",
	"site reliability",
	"sre",
	"platform engineer",
	"infrastructure engineer",
	"cloud engineer",
	"kubernetes",
	"terraform",
	"observability",
}

var securityKeywords = []string{
	"security engineer",
	"security analyst",
	"appsec",
	"application security",
	"cloud security",
	"infosec",
	"information security",
	"penetration test",
	"pentest",
	"grc",
	"compliance engineer",
	"iam engineer",
}

// Titles containing these words are excluded even if they match above.
// This drops senior roles you almost certainly can't get yet.
var excludeKeywords = []string{
	"senior",
	"principal",
	"manager",
	"director",
	"head of",
	"vp ",
}
// ─── METRIC 1: jobs found ──────────────────────────────
var jobsFound = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "internships_jobs_found_total",
		Help: "Total jobs found per company board.",
	},
	[]string{"company"},
)

// ─── METRIC 2: scrape duration ─────────────────────────
var scrapeDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "internships_scrape_duration_seconds",
		Help:    "Duration of each company board scrape.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"company"},
)

// ─── METRIC 3: scrape errors  ← ADD THIS BLOCK ─────────
var scrapeErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "internships_scrape_errors_total",
		Help: "Total scrape errors per company board.",
	},
	[]string{"company"},
)

type greenhouseResponse struct {
	Jobs []struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		Location struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
}

type matchResult struct {
	Category string // "devops", "security", or "" for no match
}

func classify(title string) matchResult {
	lower := strings.ToLower(title)

	// Exclusions first — if it's senior, we don't care what it's about.
	for _, ex := range excludeKeywords {
		if strings.Contains(lower, ex) {
			return matchResult{}
		}
	}

	for _, kw := range devopsKeywords {
		if strings.Contains(lower, kw) {
			return matchResult{Category: "devops"}
		}
	}

	for _, kw := range securityKeywords {
		if strings.Contains(lower, kw) {
			return matchResult{Category: "security"}
		}
	}

	return matchResult{}
}
func scrapeBoard(slug string) (int, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", slug)

	start := time.Now()
	defer func() {
		scrapeDuration.WithLabelValues(slug).Observe(time.Since(start).Seconds())
	}()

	resp, err := http.Get(url)
	if err != nil {
		scrapeErrors.WithLabelValues(slug).Inc()   // ← ADD
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		scrapeErrors.WithLabelValues(slug).Inc()   // ← ADD
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var payload greenhouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		scrapeErrors.WithLabelValues(slug).Inc()   // ← ADD
		return 0, err
	}

	jobsFound.WithLabelValues(slug).Add(float64(len(payload.Jobs)))
	return len(payload.Jobs), nil
}

func main() {
	boards := []string{
		"stripe", "airbnb", "anthropic", "coinbase", "robinhood",
		"figma", "notion", "databricks", "palantir", "uber",
		"lyft", "snapchat", "twitter", "reddit", "square",
		"asana", "slack", "zoom",
	}

	go func() {
		for {
			for _, slug := range boards {
				n, err := scrapeBoard(slug)
				if err != nil {
					log.Printf("scrape %s failed: %v", slug, err)
					continue
				}
				log.Printf("scrape %s: %d jobs", slug, n)
			}
			time.Sleep(6 * time.Hour)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}