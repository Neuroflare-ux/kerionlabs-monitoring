package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Gauges: "how many right now", set on every scan (unlike a counter, they don't inflate).
	boardJobs = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radar_board_jobs", Help: "Total open jobs on the board at the last scan.",
	}, []string{"board"})
	boardMatches = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radar_board_matches", Help: "Jobs on the board that passed every filter at the last scan.",
	}, []string{"board"})
	openMatches = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "radar_open_matches", Help: "Open matching jobs by category and eligibility.",
	}, []string{"category", "eligibility"})

	scanErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radar_scan_errors_total", Help: "Failed board scans.",
	}, []string{"board"})
	scanDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "radar_scan_duration_seconds", Help: "Time to fetch one board.", Buckets: prometheus.DefBuckets,
	}, []string{"board"})
	newMatches = promauto.NewCounter(prometheus.CounterOpts{
		Name: "radar_new_matches_total", Help: "Matching jobs seen for the first time.",
	})
	lastScan = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "radar_last_scan_timestamp_seconds", Help: "Unix time the last full scan finished.",
	})
)

func updateOpenMatches(jobs []Job) {
	counts := map[[2]string]int{}
	for _, j := range jobs {
		counts[[2]string{string(j.Category), string(j.Eligibility)}]++
	}
	openMatches.Reset()
	for k, n := range counts {
		openMatches.WithLabelValues(k[0], k[1]).Set(float64(n))
	}
}