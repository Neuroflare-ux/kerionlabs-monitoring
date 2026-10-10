package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Board struct{ Source, Slug string }

func (b Board) ID() string { return b.Source + ":" + b.Slug }

func loadBoards(path string) ([]Board, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var boards []Board
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		slug := ""
		if len(parts) == 2 {
			slug = strings.TrimSpace(parts[1])
		}
		if slug == "" || (parts[0] != "greenhouse" && parts[0] != "lever" && parts[0] != "ashby") {
			log.Printf("skipping bad line in %s: %q", path, line)
			continue
		}
		boards = append(boards, Board{Source: parts[0], Slug: slug})
	}
	return boards, sc.Err()
}

var showNear bool

type nearEntry struct {
	n       int
	example string
}

func printNear(locs map[string]nearEntry) {
	type row struct {
		loc string
		nearEntry
	}
	rows := make([]row, 0, len(locs))
	for l, e := range locs {
		rows = append(rows, row{loc: l, nearEntry: e})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].loc < rows[j].loc
	})
	fmt.Println("\nLocations of jobs that matched title + field but failed the location filter (top 30):")
	for i, r := range rows {
		if i == 30 {
			break
		}
		fmt.Printf("%4d  %-40s e.g. %s\n", r.n, r.loc, r.example)
	}
}

func scanAll(store *Store, boards []Board, delay time.Duration) {
	nearLocs := map[string]nearEntry{}
	for i, b := range boards {
		if i > 0 {
			time.Sleep(delay) // be polite: one board at a time
		}
		start := time.Now()
		raw, err := fetchBoard(b)
		scanDuration.WithLabelValues(b.ID()).Observe(time.Since(start).Seconds())
		if err != nil {
			scanErrors.WithLabelValues(b.ID()).Inc()
			log.Printf("%-28s ERROR: %v", b.ID(), err)
			continue
		}
		matches, f := filterJobs(b, raw)
		added := store.ApplyScan(b.ID(), matches, time.Now())
		boardJobs.WithLabelValues(b.ID()).Set(float64(f.Total))
		boardMatches.WithLabelValues(b.ID()).Set(float64(len(matches)))
		newMatches.Add(float64(added))
		log.Printf("%-28s %4d jobs -> %3d entry-level -> %3d in-field -> %3d eligible (%d new)",
			b.ID(), f.Total, f.Entry, f.InField, f.Eligible, added)
		if showNear {
			for _, nm := range f.NearMisses {
				e := nearLocs[nm.Location]
				e.n++
				if e.example == "" {
					e.example = b.Slug + ": " + nm.Title
				}
				nearLocs[nm.Location] = e
			}
		}
	}
	if showNear {
		printNear(nearLocs)
	}
	if err := store.Save(); err != nil {
		log.Printf("saving store failed: %v", err)
	}
	updateOpenMatches(store.OpenJobs())
	lastScan.SetToCurrentTime()
}

func printTable(jobs []Job) {
	if len(jobs) == 0 {
		fmt.Println("\nNo matching jobs right now. Check the funnel lines above to see which filter removed them.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "\nELIGIBILITY\tLEVEL\tFIELD\tCOMPANY\tTITLE\tLOCATION\tLINK")
	for _, j := range jobs {
		level := "-"
		if j.Entry {
			level = "entry"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", j.Eligibility, level, j.Category, j.Company, j.Title, j.Location, j.URL)
	}
	w.Flush()
	fmt.Printf("\n%d matches. REMOTE-CHECK means the posting doesn't state a country: read it before applying.\n", len(jobs))
}

type pageJob struct {
	Job
	New bool
}
type pageData struct {
	Jobs    []pageJob
	Updated string
}

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Internship Radar</title>
<style>
body{font-family:system-ui,sans-serif;margin:2rem auto;max-width:1100px;padding:0 1rem;background:#111;color:#ddd}
table{border-collapse:collapse;width:100%}th,td{text-align:left;padding:.5rem;border-bottom:1px solid #333;vertical-align:top}
a{color:#6cf}.new{background:#2a2;color:#fff;border-radius:4px;padding:0 .4rem;font-size:.75rem}
input{width:100%;padding:.5rem;margin:.5rem 0;background:#222;color:#ddd;border:1px solid #444}
.KENYA{color:#6f6}.REMOTE-GLOBAL{color:#6cf}.REMOTE-CHECK{color:#fc6}
</style></head><body>
<h1>Internship Radar</h1>
<p>{{len .Jobs}} open matches. Updated {{.Updated}}. <b>REMOTE-CHECK</b> = no country stated; read the posting before applying.</p>
<input id="q" placeholder="filter: intern, security, nairobi, remote ...">
<table><tr><th>Eligibility</th><th>Level</th><th>Field</th><th>Company</th><th>Title</th><th>Location</th><th>First seen</th></tr>
{{range .Jobs}}<tr class="job">
<td class="{{.Eligibility}}">{{.Eligibility}}</td><td>{{if .Entry}}entry{{else}}-{{end}}</td><td>{{.Category}}</td><td>{{.Company}}</td>
<td><a href="{{.URL}}" rel="noopener noreferrer" target="_blank">{{.Title}}</a> {{if .New}}<span class="new">NEW</span>{{end}}</td>
<td>{{.Location}}</td><td>{{.FirstSeen.Format "2006-01-02"}}</td></tr>
{{else}}<tr><td colspan="7">No matches yet. The first scan may still be running.</td></tr>{{end}}
</table>
<script>
var q=document.getElementById("q");
q.addEventListener("input",function(){var t=q.value.toLowerCase();
document.querySelectorAll("tr.job").forEach(function(r){r.style.display=r.textContent.toLowerCase().indexOf(t)>-1?"":"none";});});
</script></body></html>`))

// runProbe tests candidate company names against both job-board APIs and reports
// which exist, how many jobs they have, and how many are usable from Kenya.
func runProbe(path string, delay time.Duration) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("reading %s: %v", path, err)
	}
	defer f.Close()
	fmt.Printf("%-12s %-30s %6s %8s %8s\n", "SOURCE", "SLUG", "JOBS", "USABLE", "MATCHES")
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name := sc.Text()
		if i := strings.Index(name, "#"); i >= 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for _, src := range []string{"greenhouse", "lever", "ashby"} {
			b := Board{Source: src, Slug: name}
			raw, err := fetchBoard(b)
			time.Sleep(delay)
			if err != nil {
				continue // most guesses won't exist: stay quiet
			}
			usable := 0
			for _, r := range raw {
				if _, ok := eligibility(r.Location); ok {
					usable++
				}
			}
			matches, _ := filterJobs(b, raw)
			fmt.Printf("%-12s %-30s %6d %8d %8d\n", src, name, len(raw), usable, len(matches))
		}
	}
	fmt.Println("\nAdd boards with USABLE or MATCHES above 0 to boards.txt as source:slug.")
}

func main() {
	boardsPath := flag.String("boards", "boards.txt", "file listing boards to scan")
	dataPath := flag.String("data", "data/jobs.json", "where to keep job state")
	addr := flag.String("addr", ":8080", "listen address for the web page and /metrics")
	interval := flag.Duration("interval", 6*time.Hour, "time between scans")
	delay := flag.Duration("delay", time.Second, "pause between boards")
	once := flag.Bool("once", false, "scan once, print results, exit")
	probe := flag.String("probe", "", "file of company names to test against Greenhouse and Lever, then exit")
	flag.BoolVar(&looseMode, "loose", false, "skip the title/field filters: keep every job with a usable location")
	flag.BoolVar(&showNear, "near", false, "print locations of jobs that matched title + field but failed the location filter")
	flag.Parse()
	if *probe != "" {
		runProbe(*probe, *delay)
		return
	}

	boards, err := loadBoards(*boardsPath)
	if err != nil {
		log.Fatalf("reading boards: %v", err)
	}
	if len(boards) == 0 {
		log.Fatalf("no boards in %s", *boardsPath)
	}
	store, err := LoadStore(*dataPath)
	if err != nil {
		log.Fatalf("loading %s: %v", *dataPath, err)
	}

	if *once {
		scanAll(store, boards, *delay)
		printTable(store.OpenJobs())
		return
	}

	go func() {
		for {
			scanAll(store, boards, *delay)
			time.Sleep(*interval)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/jobs.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(store.OpenJobs())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		var pj []pageJob
		for _, j := range store.OpenJobs() {
			pj = append(pj, pageJob{Job: j, New: time.Since(j.FirstSeen) < 48*time.Hour})
		}
		if err := pageTmpl.Execute(w, pageData{Jobs: pj, Updated: time.Now().Format("2006-01-02 15:04")}); err != nil {
			log.Printf("render: %v", err)
		}
	})

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("listening on %s (page: /, data: /jobs.json, metrics: /metrics)", *addr)
	log.Fatal(srv.ListenAndServe())
}