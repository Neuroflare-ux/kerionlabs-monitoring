package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type RawJob struct {
	ID       string
	Title    string
	Location string
	URL      string
}

var httpClient = &http.Client{Timeout: 25 * time.Second}

func getJSON(endpoint string, out any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "internship-radar/0.1 (personal job search tool)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fetchBoard(b Board) ([]RawJob, error) {
	switch b.Source {
	case "greenhouse":
		return fetchGreenhouse(b.Slug)
	case "lever":
		return fetchLever(b.Slug)
	case "ashby":
		return fetchAshby(b.Slug)
	}
	return nil, fmt.Errorf("unknown source %q", b.Source)
}

// Greenhouse public Job Board API: no key needed, returns the whole board in one response.
func fetchGreenhouse(slug string) ([]RawJob, error) {
	var p struct {
		Jobs []struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			AbsoluteURL string `json:"absolute_url"`
			Location    struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"jobs"`
	}
	endpoint := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(slug) + "/jobs"
	if err := getJSON(endpoint, &p); err != nil {
		return nil, err
	}
	out := make([]RawJob, 0, len(p.Jobs))
	for _, j := range p.Jobs {
		out = append(out, RawJob{
			ID:       strconv.FormatInt(j.ID, 10),
			Title:    j.Title,
			Location: j.Location.Name,
			URL:      j.AbsoluteURL,
		})
	}
	return out, nil
}

// Lever public postings API (written from memory of the format: verify with your first run).
// EU-hosted Lever accounts use api.eu.lever.co instead.
func fetchLever(slug string) ([]RawJob, error) {
	var p []struct {
		ID            string `json:"id"`
		Text          string `json:"text"`
		HostedURL     string `json:"hostedUrl"`
		WorkplaceType string `json:"workplaceType"`
		Country       string `json:"country"`
		Categories    struct {
			Location string `json:"location"`
		} `json:"categories"`
	}
	endpoint := "https://api.lever.co/v0/postings/" + url.PathEscape(slug) + "?mode=json"
	if err := getJSON(endpoint, &p); err != nil {
		return nil, err
	}
	out := make([]RawJob, 0, len(p))
	for _, j := range p {
		loc := j.Categories.Location
		if j.WorkplaceType == "remote" {
			loc += " (Remote) " + j.Country
		}
		out = append(out, RawJob{ID: j.ID, Title: j.Text, Location: loc, URL: j.HostedURL})
	}
	return out, nil
}

// Ashby public posting API. Field names are written from memory: check them with -probe.
func fetchAshby(slug string) ([]RawJob, error) {
	var p struct {
		Jobs []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Location string `json:"location"`
			IsRemote bool   `json:"isRemote"`
			JobURL   string `json:"jobUrl"`
		} `json:"jobs"`
	}
	endpoint := "https://api.ashbyhq.com/posting-api/job-board/" + url.PathEscape(slug)
	if err := getJSON(endpoint, &p); err != nil {
		return nil, err
	}
	out := make([]RawJob, 0, len(p.Jobs))
	for _, j := range p.Jobs {
		loc := j.Location
		if j.IsRemote {
			loc += " (Remote)"
		}
		out = append(out, RawJob{ID: j.ID, Title: j.Title, Location: loc, URL: j.JobURL})
	}
	return out, nil
}