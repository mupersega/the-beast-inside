// The Beast Inside - single-binary, server-rendered site (Go + html/template + htmx).
//
//	go run .      # then open http://localhost:8080
package main

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Block is one segment of a session (warm-up, main lift, …).
type Block struct {
	Name   string
	Detail string
}

// Day is one of the three weekly sessions. A day with no Blocks is "coming
// soon" and renders Soon instead.
type Day struct {
	Num     int
	Name    string // "Day One"
	Tool    string // "Barbells"
	Title   string // headline shown in the panel
	Tagline string
	Blocks  []Block
	Note    string
	Soon    string
}

// days holds the three sessions in order: Kettlebell, Barbell, Body Work.
// The hero circles show each day's Tagline; the week-by-week detail lives in the
// phases below (taken from Sam's program outline).
var days = map[int]Day{
	1: {
		Num: 1, Name: "Day One", Tool: "Kettlebell", Title: "Kettlebell",
		Tagline: "Kettlebell work - two-hand swings and Turkish get-ups, building to Simple & Sinister.",
		Soon:    "Full session details coming soon.",
	},
	2: {
		Num: 2, Name: "Day Two", Tool: "Barbell", Title: "Barbell",
		Tagline: "The three big lifts - deadlift, Zercher squat and military press - built to a heavy 3RM.",
		Soon:    "Full session details coming soon.",
	},
	3: {
		Num: 3, Name: "Day Three", Tool: "Body Work", Title: "Body Work",
		Tagline: "Bodyweight strength - push-ups, pull-ups and squats - full-body connection and control.",
		Soon:    "Full session details coming soon.",
	},
}

// dayList is the three sessions in order, for rendering the hero circles.
var dayList = []Day{days[1], days[2], days[3]}

// homeData is everything the landing page template needs.
type homeData struct {
	Days       []Day
	Phases     []Phase
	Spots      []bool // one per place; true = taken
	SpotsLeft  int    // -1 = unknown (placeholder); otherwise 0..SpotsTotal
	SpotsTotal int
}

const spotsTotal = 8

// spotsLeft is how many places remain in the next intake. -1 shows a clearly
// marked placeholder; Sam sets this to a real number (0..8) when an intake opens.
var spotsLeft = -1

// Phase is one stage of the 8-week arc (Week 1, Week 2, Weeks 3-7, Week 8).
// Detail is that week's three days (kettlebell, barbell, body weight), taken
// from Sam's program outline.
type Phase struct {
	Num     int
	Weeks   string // "Week 1", "Weeks 3-7"
	Label   string // "Phase One"
	Name    string // "The Wake"
	Tagline string
	Detail  []Block
}

var phases = []Phase{
	{
		Num: 1, Weeks: "Week 1", Label: "Phase One", Name: "Foundation",
		Tagline: "Learn the lifts and set your starting numbers.",
		Detail: []Block{
			{"Kettlebell", "Learn the two-hand swing and the Turkish get-up. Finish with a simple test."},
			{"Barbell", "Learn the three big lifts - deadlift, Zercher squat and military press. Finish by finding your 3RM."},
			{"Body weight", "Learn full-body connection and the push-up, pull-up and squat / split squat. Finish with a max test."},
		},
	},
	{
		Num: 2, Weeks: "Week 2", Label: "Phase Two", Name: "Practice",
		Tagline: "Recap the foundations, then put them into practice.",
		Detail: []Block{
			{"Kettlebell", "Recap the swing and the get-up, then put them into practice."},
			{"Barbell", "Recap the deadlift, Zercher squat and military press, then put them into practice."},
			{"Body weight", "Recap the push-up, pull-up and squat patterns, then put them into practice."},
		},
	},
	{
		Num: 3, Weeks: "Weeks 3-7", Label: "Phase Three", Name: "Overload",
		Tagline: "The program proper - a little more each week.",
		Detail: []Block{
			{"Kettlebell", "Simple & Sinister."},
			{"Barbell", "Building to a new 3RM."},
			{"Body weight", "Increasing volume and/or skill in the movements."},
		},
	},
	{
		Num: 4, Weeks: "Week 8", Label: "Finish", Name: "Test week",
		Tagline: "Retest your week-one numbers and see the progress.",
		Detail: []Block{
			{"Kettlebell", "Timed Simple."},
			{"Barbell", "3-rep-max retest - deadlift, Zercher squat and military press."},
			{"Body weight", "Retest the movements - push-up, pull-up and squat or split squat."},
		},
	},
}

func main() {
	// `go run . build` (or the built binary with `build`) renders the site to
	// ./dist as plain static files - no server needed to host it.
	if len(os.Args) > 1 && os.Args[1] == "build" {
		if err := buildStatic(); err != nil {
			log.Fatalf("build: %v", err)
		}
		log.Println("static site written to ./dist  (deploy that folder)")
		return
	}

	mux := http.NewServeMux()

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	// no-cache so edited CSS/JS/SVG always re-fetch (embed.FS sends no validators,
	// so browsers would otherwise serve stale assets from heuristic cache).
	staticFiles := http.StripPrefix("/static/", http.FileServer(http.FS(staticSub)))
	mux.Handle("/static/", noCache(staticFiles))

	mux.HandleFunc("/", index)
	mux.HandleFunc("/day/", day)
	mux.HandleFunc("/phase/", phase)
	mux.HandleFunc("/enquire", enquire)

	addr := ":" + port()
	srv := &http.Server{Handler: mux}

	// Bind with a short retry so a live-reload restart waits for the previous
	// process to release the port instead of crashing on "address in use".
	ln, err := listen(addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	go func() {
		log.Printf("The Beast Inside - listening on http://localhost%s", addr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	// Graceful shutdown: when Air (or Ctrl+C) signals, close the listener
	// promptly so the port is free for the rebuilt binary.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// listen binds addr, retrying briefly so a live-reload restart can wait out the
// previous process releasing the port rather than failing immediately.
func listen(addr string) (net.Listener, error) {
	deadline := time.Now().Add(6 * time.Second)
	for {
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	spots := make([]bool, spotsTotal)
	if spotsLeft >= 0 {
		for i := 0; i < spotsTotal-spotsLeft && i < spotsTotal; i++ {
			spots[i] = true // earliest pips are "taken"
		}
	}
	render(w, "index.html", homeData{
		Days: dayList, Phases: phases,
		Spots: spots, SpotsLeft: spotsLeft, SpotsTotal: spotsTotal,
	})
}

func day(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/day/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, ok := days[n]
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, "day", d)
}

func phase(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/phase/"))
	if err != nil || n < 1 || n > len(phases) {
		http.NotFound(w, r)
		return
	}
	render(w, "phase", phases[n-1])
}

// enquire handles the "get in touch" form. Placeholder: it validates lightly
// and returns a confirmation. TODO: persist the enquiry / email Sam - the other
// fields (phone, age, experience, goal, availability[], commitment checkboxes)
// are accepted but not yet stored. availability is multi-value: r.Form["availability"].
func enquire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "mate"
	}
	// first name only, for a friendlier reply
	if i := strings.IndexByte(name, ' '); i > 0 {
		name = name[:i]
	}
	render(w, "enquire", name)
}

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// noCache tells browsers to revalidate static assets every load. embed.FS sends
// no ETag/Last-Modified, so without this a browser may serve stale CSS/JS.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		h.ServeHTTP(w, r)
	})
}

// buildStatic renders index.html and copies the embedded static assets into
// ./dist - a self-contained static site you can host anywhere (GitHub Pages,
// Cloudflare Pages, Netlify, …). The page is fully client-side now: the phase
// panels toggle in JS and the enquiry form posts to Web3Forms.
func buildStatic() error {
	spots := make([]bool, spotsTotal)
	if spotsLeft >= 0 {
		for i := 0; i < spotsTotal-spotsLeft && i < spotsTotal; i++ {
			spots[i] = true
		}
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "index.html", homeData{
		Days: dayList, Phases: phases,
		Spots: spots, SpotsLeft: spotsLeft, SpotsTotal: spotsTotal,
	}); err != nil {
		return err
	}
	if err := os.MkdirAll("dist", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("dist", "index.html"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	// stop GitHub Pages running the output through Jekyll
	if err := os.WriteFile(filepath.Join("dist", ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	// custom domain for GitHub Pages (keeps the domain set across Actions deploys)
	if err := os.WriteFile(filepath.Join("dist", "CNAME"), []byte("beastinside.com.au\n"), 0o644); err != nil {
		return err
	}
	// copy embedded static/* into dist/static/*
	return fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out := filepath.Join("dist", filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := staticFS.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
}
