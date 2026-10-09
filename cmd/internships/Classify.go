package main

import (
	"regexp"
	"sort"
)

type Category string
type Eligibility string

const (
	CatCloudDevOps Category = "cloud-devops"
	CatSecurity    Category = "security"
	CatITSupport   Category = "it-support"
	CatSoftware    Category = "software"

	EligKenya        Eligibility = "KENYA"
	EligRemoteGlobal Eligibility = "REMOTE-GLOBAL"
	EligRemoteCheck  Eligibility = "REMOTE-CHECK" // remote, but no country stated: read the posting
)

// Entry-level signals in a job title.
var entryRe = regexp.MustCompile(`(?i)\b(intern|internship|trainee|apprentice|apprenticeship|graduate|new grad|early career|entry[- ]level|junior|jr|associate|co-op)\b|\b(engineer|developer|analyst|administrator|specialist|technician) (i|1)\b`)

// Seniority signals: these always disqualify, even if an entry word is present.
var seniorRe = regexp.MustCompile(`(?i)\b(senior|sr|staff|principal|lead|manager|director|head of|vp|vice president|architect|distinguished)\b`)

// First match wins, so order = priority.
var categoryRules = []struct {
	cat Category
	re  *regexp.Regexp
}{
	{CatCloudDevOps, regexp.MustCompile(`(?i)devops|dev ops|site reliability|\bsre\b|platform engineer|infrastructure|\bcloud\b|kubernetes|terraform|observability|systems? (engineer|administrator)|sysadmin|network engineer|reliability engineer`)},
	{CatSecurity, regexp.MustCompile(`(?i)security|appsec|infosec|penetration|pentest|\bgrc\b|compliance|\bsoc\b|incident response|threat|\biam\b`)},
	{CatITSupport, regexp.MustCompile(`(?i)\bit (support|technician|analyst|specialist|administrator|intern)|help ?desk|technical support|support engineer|desktop support`)},
	{CatSoftware, regexp.MustCompile(`(?i)software|developer|programmer|backend|back-end|front-?end|full-?stack|data engineer|data analyst|machine learning|\bml\b|engineering intern`)},
}

var (
	kenyaRe   = regexp.MustCompile(`(?i)\b(kenya|nairobi|mombasa|kisumu|ke)\b`)
	remoteRe  = regexp.MustCompile(`(?i)\b(remote|anywhere|work from home|wfh|distributed)\b`)
	globalRe  = regexp.MustCompile(`(?i)\b(worldwide|anywhere|global|globally|international|africa)\b`)
	restrictRe = regexp.MustCompile(`(?i)\b(us|usa|u\.s|united states|canada|uk|united kingdom|germany|france|spain|ireland|india|brazil|mexico|australia|singapore|japan|netherlands|poland|israel|europe|apac|latam|north america|ny|nyc|ca|tx|wa|ma|il|co|fl|va|dc|nc|ga|az|oh|pa|nj|mn|ut|mi)\b`)
)

func isEntryLevel(title string) bool {
	if seniorRe.MatchString(title) {
		return false
	}
	return entryRe.MatchString(title)
}

func categorize(title string) Category {
	for _, r := range categoryRules {
		if r.re.MatchString(title) {
			return r.cat
		}
	}
	return ""
}

// eligibility decides whether a location is usable from Kenya.
// It is a heuristic: REMOTE-CHECK means "remote, country unstated".
func eligibility(location string) (Eligibility, bool) {
	if kenyaRe.MatchString(location) {
		return EligKenya, true
	}
	if !remoteRe.MatchString(location) {
		return "", false
	}
	if globalRe.MatchString(location) {
		return EligRemoteGlobal, true
	}
	if restrictRe.MatchString(location) {
		return "", false
	}
	return EligRemoteCheck, true
}

type Funnel struct{ Total, Entry, InField, Eligible int }

func filterJobs(b Board, raw []RawJob) ([]Job, Funnel) {
	f := Funnel{Total: len(raw)}
	var out []Job
	for _, r := range raw {
		if !isEntryLevel(r.Title) {
			continue
		}
		f.Entry++
		cat := categorize(r.Title)
		if cat == "" {
			continue
		}
		f.InField++
		elig, ok := eligibility(r.Location)
		if !ok {
			continue
		}
		f.Eligible++
		out = append(out, Job{
			Key:         b.ID() + ":" + r.ID,
			Board:       b.ID(),
			Company:     b.Slug,
			Title:       r.Title,
			Location:    r.Location,
			URL:         r.URL,
			Category:    cat,
			Eligibility: elig,
		})
	}
	return out, f
}

var eligRank = map[Eligibility]int{EligKenya: 0, EligRemoteGlobal: 1, EligRemoteCheck: 2}
var catRank = map[Category]int{CatCloudDevOps: 0, CatSecurity: 1, CatITSupport: 2, CatSoftware: 3}

// sortJobs: Kenya first, then best-fit field, then company and title.
func sortJobs(jobs []Job) {
	sort.Slice(jobs, func(i, j int) bool {
		a, b := jobs[i], jobs[j]
		if eligRank[a.Eligibility] != eligRank[b.Eligibility] {
			return eligRank[a.Eligibility] < eligRank[b.Eligibility]
		}
		if catRank[a.Category] != catRank[b.Category] {
			return catRank[a.Category] < catRank[b.Category]
		}
		if a.Company != b.Company {
			return a.Company < b.Company
		}
		return a.Title < b.Title
	})
}