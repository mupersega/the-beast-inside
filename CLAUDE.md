# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**The Beast Inside** — a single-page marketing/landing site for Sam Barker's 8-week
strength program (Studio 99 Fitness, West End Brisbane; men aged 40–50). It is a
single self-contained Go binary: server-rendered `html/template` HTML, progressively
enhanced with htmx 2.x and a little vanilla JS. No database, no third-party Go modules
(`go.mod` has no `require` block) — htmx is vendored as `static/htmx.min.js`.

Much of the copy is intentional **placeholder**, marked with `[ bracketed ]` text and
`TODO` comments, that Sam fills in before launch (prices, dates, session times, his own
bio, real `spotsLeft`). Don't "fix" these by inventing facts — leave the markers unless
asked to supply real content.

## Commands

```sh
air                                  # dev: live-reload, open http://localhost:8090
go build -o tbi.exe . && ./tbi.exe   # prod build, serves http://localhost:8080
go run .                             # run without Air, http://localhost:8080
go vet ./...                         # there are no tests in this repo
PORT=9000 go run .                   # PORT env overrides the :8080 default
```

There is no test suite, lint config, or CI. To verify a change, run the app and look at
it (Playwright MCP, or the allow-listed `curl` healthcheck against `:8080`).

## The embed gotcha (read this before editing HTML/CSS)

`templates/*.html` and `static/` are baked into the binary with `go:embed` (see the
directives at the top of `main.go`). **Editing a `.html`, `.css`, or asset file has no
effect until the binary is rebuilt** — the running server still serves the old embedded
copy. In dev, Air handles this: `.air.toml` watches those extensions and triggers a
rebuild + browser refresh through its proxy on **:8090** (open that, not :8080).

Because `embed.FS` sends no ETag/Last-Modified, `main.go`'s `noCache` wrapper forces
revalidation of `/static/` so browsers don't serve stale CSS/JS after a rebuild.

On Windows, Air's rebuild can race the previous process for the port. `main.go` handles
this deliberately: `listen()` retries the bind for ~6s, and a graceful-shutdown handler
releases `:8080` promptly on SIGTERM. Keep both if you touch server startup.

## Architecture

Everything server-side lives in **`main.go`** (~290 lines):

- **Content is Go data, not a CMS.** The page's content is hardcoded as package-level
  values: `days` (the 3 weekly sessions), `phases` (the 4 stages of the 8-week arc),
  and `spotsTotal` / `spotsLeft` (intake availability; `spotsLeft = -1` renders a
  "set me" placeholder). To change page copy/structure, edit these values — and the
  matching template — together.
- **Routes** (`net/http`, stdlib mux): `/` (`index`), `/day/{n}` and `/phase/{n}`
  (htmx partials), and `POST /enquire` (the lead form). All handlers funnel through
  `render()`, which executes a named template.
- **`enquire` is a stub.** It validates lightly and returns a thank-you fragment but
  **does not persist or email** the enquiry yet (see the TODO in the handler). All form
  fields are accepted; `availability` is multi-value (`r.Form["availability"]`).

### Templates (`templates/`)

`html/template`, all parsed at startup into one set. `index.html` is the full page;
the others are named partials returned to htmx:

- `phase.html` (`{{define "phase"}}`) → swapped into `#phase-panel` when a phase button
  is clicked (`hx-get="/phase/{n}"`).
- `enquire.html` → replaces the lead form on submit (`hx-post="/enquire"`,
  `hx-swap="outerHTML"`).
- `dayshape.html` → an inline SVG with a single morphing `<path>`; the resting shape is
  a circle that CSS animates (via `d` transitions) into a barbell / kettlebell / human
  silhouette. Paths are machine-generated — don't hand-edit the coordinates.
- `day.html` / the `/day/` route exist but the hero day-circles are currently driven by
  the JS in `index.html`, not htmx, so this partial is effectively unused.

### Front-end (`static/app.css`, inline JS in `index.html`)

- **Design language:** near-black (`--bg #141516`) + white + a warm tan accent
  (`--accent #cba867`), flat and high-contrast, self-hosted **Oswald** variable font.
  All styling is one sectioned file, `static/app.css` (~650 lines, banner-comment
  sections). CSS custom properties in `:root` are the theme knobs.
- **Three inline `<script>` blocks at the bottom of `index.html`**, all vanilla,
  dependency-free, and gated on `prefers-reduced-motion`: (1) IntersectionObserver
  scroll-reveal; (2) the hero day-circles' auto-cycle / hover / tap-to-pin behavior;
  (3) a WebGL fog/mist fragment shader painted into the header `<canvas>` (with a CSS
  gradient fallback if WebGL is unavailable).
- Accessibility is intentional throughout (skip link, ARIA on the interactive circles,
  reduced-motion guards) — preserve it when editing.
