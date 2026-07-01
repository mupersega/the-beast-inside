# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**The Beast Inside** — a single-page marketing/landing site for Sam Barker's 8-week
strength program (Studio 99 Fitness, 993 Stanley St, East Brisbane, Brisbane; men aged
40–50).

It began as a server-rendered Go binary and is now **also a static-site generator**:
`go run . build` renders the page to `./dist`, which is deployed to **GitHub Pages**.
The deployed site is fully client-side — vanilla JS only (htmx was removed), no
database, no third-party Go modules (`go.mod` has no `require` block). Fonts are
self-hosted; the only external runtime requests are the lead form's POST to Web3Forms
and a keyless Google Maps embed in the Details section.

Live: **https://mupersega.github.io/the-beast-inside/** (auto-deploys on push to `main`).

## Real content vs placeholders

The **week-by-week program is real** — Sam's actual plan lives in `phases` in `main.go`
(Week 1 Foundation → Week 8 Test week, each broken into the three days). Don't treat it
as scaffolding.

Still intentional **placeholder**, marked with `[ bracketed ]` text — price, dates/times,
session length, Sam's bio, real `spotsLeft`. Don't "fix" these by inventing facts; leave
the markers unless given real content. `og:image` / canonical URL are placeholders until
the domain is known.

## Commands

```sh
air                                  # dev: live-reload, open http://localhost:8090
go run .                             # run the Go server, http://localhost:8080
go run . build                       # render the static site to ./dist (the deployed artifact)
go build -o tbi.exe . && ./tbi.exe   # prod build of the server, :8080
go vet ./...                         # there are no tests in this repo
PORT=9000 go run .                   # PORT env overrides the :8080 default
```

Deploy is automatic: push to `main` → `.github/workflows/deploy.yml` runs `go run . build`
and publishes `dist/` to GitHub Pages (see `DEPLOY.md`). No test suite; `go vet` is the
only check. To verify a change, run the app and look at it (Playwright MCP).

## The embed gotcha (read this before editing HTML/CSS)

`templates/*.html` and `static/` are baked into the binary with `go:embed` (see the
directives at the top of `main.go`). **Editing a `.html`, `.css`, or asset file has no
effect until the binary is rebuilt** — the running server (and `go run . build`) keep
using the old embedded copy until recompiled. In dev, Air handles this: `.air.toml`
watches those extensions and triggers a rebuild + browser refresh through its proxy on
**:8090** (open that, not :8080).

Because `embed.FS` sends no ETag/Last-Modified, `main.go`'s `noCache` wrapper forces
revalidation of `/static/` so the dev server doesn't serve stale CSS/JS after a rebuild.

On Windows, Air's rebuild can race the previous process for the port. `main.go` handles
this deliberately: `listen()` retries the bind for ~6s, and a graceful-shutdown handler
releases `:8080` promptly on SIGTERM. Keep both if you touch server startup.

## Architecture

Everything server-side lives in **`main.go`** (~310 lines):

- **Content is Go data, not a CMS.** Page content is hardcoded as package-level values:
  `days` (the 3 weekly sessions, in order **Kettlebell · Barbell · Body Work**), `phases`
  (the 4 weeks of the 8-week arc, each carrying that week's three days from Sam's plan),
  and `spotsTotal` / `spotsLeft` (intake availability; `spotsLeft = -1` renders a "set me"
  placeholder). To change copy/structure, edit these values **and** the matching template
  together.
- **`go run . build` → `buildStatic()`** renders `index.html` to `dist/index.html`, writes
  `.nojekyll`, and copies the embedded `static/` tree into `dist/static/`. That folder is
  the deployable artifact.
- **Server routes** (`net/http`, stdlib mux) are **dev-only**: `/` (`index`), `/day/{n}`
  and `/phase/{n}` (partials), `POST /enquire`. The deployed static site does not use them
  — phase panels toggle in client JS and the form posts to Web3Forms.
- **`enquire` is a stub** (dev only): validates lightly, returns a thank-you fragment, does
  not persist or email. The real (static) form goes to Web3Forms.

### Templates (`templates/`)

`html/template`, all parsed at startup into one set. `index.html` is the full page; the
rest are named partials:

- `phase.html` (`{{define "phase"}}`) — one week's panel (its three days). All four phases
  are rendered **inline** in `index.html` and shown/hidden by a small JS toggle (was an
  htmx `hx-get`; htmx is gone).
- `dayshape.html` — inline SVG with a single morphing `<path>`; a resting circle that CSS
  animates (via `d` transitions) into the day's silhouette. Order matches `days`:
  **c-tl = kettlebell, c-tr = barbell, c-b = the running figure**. Paths are
  machine-generated — don't hand-edit the coordinates.
- `enquire.html` / `day.html` — used only by the dev server routes; effectively unused by
  the deployed static page.

### Front-end (`static/app.css`, inline JS in `index.html`)

- **Design language:** near-black (`--bg #141516`) + white + a warm tan accent
  (`--accent #cba867`), flat and high-contrast. Two self-hosted variable fonts: **Oswald**
  (`--head`) and **Caveat** (`--script`, the handwritten "writing" font used for "never
  left" and the spots callout — self-hosted in `static/fonts/` so it renders identically on
  every device). All styling is one sectioned file, `static/app.css` (~730 lines). CSS
  custom properties in `:root` are the theme knobs.
- **Header** is sticky and click-to-top (the wordmark is a real `#top` link; clicking
  elsewhere on the bar scrolls up). The right side is a translucent, fog-blended **Apply**
  chip with an occasional sheen sweep. "NEVER LEFT" (Caveat) sits beside the wordmark and
  is hidden on small phones. Section anchors carry `scroll-margin-top` so jumps clear the
  sticky bar.
- **Seven inline `<script>` blocks** at the bottom of `index.html`, all vanilla,
  dependency-free, most gated on `prefers-reduced-motion`: scroll-reveal
  (IntersectionObserver); the hero day-circles (auto-cycle / hover / tap-to-pin); the WebGL
  fog shader in the header `<canvas>` (declares **`highp`** so it renders on mobile —
  `mediump` overflowed the noise hash on real phone GPUs and collapsed the fog; CSS-gradient
  fallback if WebGL is unavailable); the phase-panel toggle; header click-to-top; and the
  Web3Forms form submit (AJAX + inline confirmation).
- The lead form posts to **Web3Forms** (the `access_key` is set in `index.html`; Web3Forms
  keys are public by design). The age input is validated to **40–50**.
- Accessibility is intentional throughout (skip link, ARIA on the circles, reduced-motion
  guards, `-webkit-tap-highlight-color: transparent` to kill the mobile blue tap-flash) —
  preserve it when editing.

### Deploy

GitHub Pages via `.github/workflows/deploy.yml` (repo `mupersega/the-beast-inside`): on push
to `main` it runs `go run . build` and publishes `dist/`. Relative asset paths make it work
on the `/the-beast-inside/` project sub-path. `DEPLOY.md` has the full steps; it's also fine
on Cloudflare Pages / Netlify (build `go run . build`, output `dist`).
