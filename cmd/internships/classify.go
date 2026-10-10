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
	CatOther       Category = "other" // only appears in -loose mode

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
	emeaRe     = regexp.MustCompile(`(?i)\bemea\b`)
	fillerRe   = regexp.MustCompile(`(?i)\b(remote|anywhere|work from home|wfh|distributed|or|and|hybrid|only|based|friendly|eligible)\b`)
	nonLetters = regexp.MustCompile(`[^a-zA-Z]+`)
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
// Rule: a remote job is only kept if the location names NOTHING besides "remote"
// (REMOTE-CHECK), or explicitly says worldwide/Africa/EMEA. Any other place name
// (a country, a city, a state code) means it is restricted, so it is dropped.
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
	if emeaRe.MatchString(location) {
		return EligRemoteCheck, true
	}
	rest := nonLetters.ReplaceAllString(fillerRe.ReplaceAllString(location, " "), "")
	if rest == "" {
		return EligRemoteCheck, true
	}
	return "", false
}

// looseMode keeps every job with a usable location, whatever its title.
var looseMode bool

type NearMiss struct{ Title, Location string }

type Funnel struct {
	Total, Entry, InField, Eligible int
	NearMisses                      []NearMiss // passed title + field, failed location
}

func filterJobs(b Board, raw []RawJob) ([]Job, Funnel) {
	f := Funnel{Total: len(raw)}
	var out []Job
	for _, r := range raw {
		entry := isEntryLevel(r.Title)
		cat := categorize(r.Title)
		strict := entry && cat != "" // the original title + field test
		if entry {
			f.Entry++
		}
		if strict {
			f.InField++
		}
		if !looseMode && !strict {
			continue
		}
		elig, ok := eligibility(r.Location)
		if !ok {
			if strict {
				f.NearMisses = append(f.NearMisses, NearMiss{Title: r.Title, Location: r.Location})
			}
			continue
		}
		f.Eligible++
		if cat == "" {
			cat = CatOther
		}
		out = append(out, Job{
			Key:         b.ID() + ":" + r.ID,
			Board:       b.ID(),
			Company:     b.Slug,
			Title:       r.Title,
			Location:    r.Location,
			URL:         r.URL,
			Category:    cat,
			Eligibility: elig,
			Entry:       entry,
		})
	}
	return out, f
}

var eligRank = map[Eligibility]int{EligKenya: 0, EligRemoteGlobal: 1, EligRemoteCheck: 2}
var catRank = map[Category]int{CatCloudDevOps: 0, CatSecurity: 1, CatITSupport: 2, CatSoftware: 3, CatOther: 4}

// sortJobs: Kenya first, then best-fit field, then company and title.
func sortJobs(jobs []Job) {
	sort.Slice(jobs, func(i, j int) bool {
		a, b := jobs[i], jobs[j]
		if eligRank[a.Eligibility] != eligRank[b.Eligibility] {
			return eligRank[a.Eligibility] < eligRank[b.Eligibility]
		}
		if a.Entry != b.Entry {
			return a.Entry // entry-level first
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