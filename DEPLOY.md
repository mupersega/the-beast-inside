# Deploying The Beast Inside

The site is a **static page** built from the Go templates. There is no server to
host — `go run . build` renders everything into `./dist`, and that folder is the
whole site. The included GitHub Actions workflow does this for you on every push.

```
go run . build      # writes ./dist  (index.html + static/)
```

Dev is unchanged: `air` → http://localhost:8090 (live reload while you edit).

---

## 1. Set the form's email key (Web3Forms)

The enquiry form emails each submission via **Web3Forms** — free, no account or
dashboard, works on a static host.

1. Go to <https://web3forms.com>, enter the email address where Sam wants
   enquiries to land, and copy the **Access Key** it gives you.
2. In `templates/index.html`, replace `YOUR_WEB3FORMS_ACCESS_KEY` with that key.
3. Rebuild (`go run . build`) / commit + push.

Until the key is set, the form will show a "didn't send" error on submit.

---

## 2. Publish to GitHub Pages (recommended)

`git init` and the first commit are already done locally. Then:

```
git remote add origin https://github.com/<you>/<repo>.git
git push -u origin main
```

In the new repo: **Settings → Pages → Build and deployment → Source: GitHub
Actions**. The workflow in `.github/workflows/deploy.yml` builds and deploys on
every push to `main`. Your site lands at `https://<you>.github.io/<repo>/`.

Asset paths are relative, so it works at that `/<repo>/` sub-path as-is.

### Other static hosts (also fine)

- **Cloudflare Pages / Netlify:** connect the repo with build command
  `go run . build` and output directory `dist` — or just drag-and-drop the `dist`
  folder. These serve from a root domain (`*.pages.dev`, `*.netlify.app`).

---

## 3. Before you send people to it

- **Fill the placeholders** (they're intentionally marked, not real facts):
  `[ Price ]`, `[ Next intake ]`, `[ Days & times ]`, `[ session length ]`,
  `[ Sam: set spots left ]`, and Sam's draft bio in the trainer section.
- **Social preview:** once you know the domain, set `og:image`, `twitter:image`
  and add a canonical URL using the full `https://…` address (scrapers need
  absolute URLs). They're in the `<head>` of `templates/index.html`.
- Optionally set a real **spots left** count: `var spotsLeft` in `main.go`
  (`-1` shows the placeholder; `0–8` shows "N of 8 spots left").
