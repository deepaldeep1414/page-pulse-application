# Page Pulse

A small web tool that audits any URL: fetch the page, report on HTTP status,
timing, SEO basics, and accessibility gaps, and never crash on bad input.

Built for the Digital Heroes SDE internship qualification task (Task A + B).

## What it does

Paste a URL in, get back:

- HTTP status code
- Response time (ms)
- Page `<title>`
- Meta description
- Number of `<h1>` tags
- Image count, and how many are missing `alt` text
- Approximate word count

Bad input (invalid URL, timeout, unreachable host, non-HTML response) comes
back as a clear JSON error with an appropriate HTTP status — the server
never panics.

## Setup

Requires Go 1.21+.

```bash
git clone <this-repo>
cd pagepulse
go run .
# Page Pulse listening on :8080
```

Open `http://localhost:8080` for the UI, or hit the API directly:

```bash
curl "http://localhost:8080/api/audit?url=https://example.com"
```

### Running the tests

```bash
go test ./... -v
```

### Deploying (free tier)

The app is a single static Go binary with no external dependencies and no
database, so it fits almost any free-tier host. Two easy options:

**Render.com (Web Service)**
1. Push this repo to GitHub.
2. New → Web Service → connect the repo.
3. Build command: `go build -o app .` — Start command: `./app`.
4. Render sets `PORT` automatically; the app reads it from the environment.

**Fly.io**
1. `fly launch` (accept the Go detection).
2. `fly deploy`.

Either way, the deployed URL is what goes in your submission as the "Live
deployed link."

## API contract

### `GET /api/audit?url=<url>`

or

### `POST /api/audit` with body `{"url": "<url>"}`

**Success — `200 OK`** (or whatever status the target page returned, in
`statusCode`):

```json
{
  "url": "https://example.com",
  "statusCode": 200,
  "responseTimeMs": 187,
  "title": "Example Domain",
  "metaDescription": "",
  "h1Count": 1,
  "imagesTotal": 0,
  "imagesMissingAlt": 0,
  "wordCount": 28
}
```

**Failure** — status code depends on the failure, body is always:

```json
{ "error": "human-readable message", "kind": "invalid_url" }
```

| HTTP status | `kind`           | Meaning                                   |
|-------------|------------------|--------------------------------------------|
| 400         | `invalid_url`    | Missing, malformed, or non-http(s) URL      |
| 400         | `invalid_request`| POST body isn't valid JSON with a `url`     |
| 422         | `non_html`       | Target responded with a non-HTML content-type |
| 502         | `unreachable`    | DNS failure, connection refused, etc.       |
| 504         | `timeout`        | Request exceeded the 10s server-side timeout |
| 500         | `internal_error` | Unexpected server error (backstopped by a recover middleware) |
| 405         | `method_not_allowed` | Anything other than GET/POST/OPTIONS    |

`GET /healthz` returns `200 ok` for uptime checks.

## Design decisions

**1. Regex-based HTML parsing instead of a full HTML parser library.**
The task's parsing needs (title, one meta tag, h1 count, alt attributes,
rough word count) are all shallow, tag-level extractions — they don't need
a DOM. Pulling in `golang.org/x/net/html` (or similar) would add a
dependency and a `go.sum` for a job that a handful of targeted regexes do
just as well. The tradeoff is honestly stated: this parser will mis-handle
deeply pathological/malformed HTML that a real tree parser would recover
from gracefully. For an audit tool reading normal, roughly well-formed
pages, that tradeoff is worth it, and it means `go build` produces a
single static binary with zero external dependencies — trivial to build
and deploy anywhere.

**2. A single shared `http.Client` with a hard 10-second timeout, not a
per-request context.** Every outbound fetch goes through one client
configured once at startup. This makes the timeout behavior uniform and
impossible to forget on a new code path, and it's what actually produces
the `timeout` error kind the tests assert on. The cost is that all targets
get the same timeout regardless of how slow-but-legitimate they are; if
this needed to support, say, large PDFs disguised as slow HTML, a
per-request configurable timeout would be worth the extra complexity. For
an audit tool checking arbitrary public pages, a fixed ceiling is the
safer default.

**3. Errors are a typed `*AuditError` with a `Kind` field, not raw Go
errors bubbled up as strings.** The HTTP layer (`statusForError` in
`main.go`) maps `Kind` to both an HTTP status code and a stable machine-
readable string in the JSON body. This means the frontend (or any other
client) can branch on `kind` without parsing English error text, and it
keeps `FetchAndAudit` fully testable in isolation — tests assert on
`Kind`, not on message wording. The alternative (returning `error` and
inspecting `strings.Contains` on the message everywhere) is what this
was written specifically to avoid; that pattern breaks the moment a
wrapped library error's wording changes.

## What I'd change with another day

The regex HTML parsing (design decision #1) is the thing I'd revisit
first. It's fine for the happy path and for most real-world pages, but a
page with a commented-out `<title>` tag, or an `<h1>` split across
malformed nesting, will fool it in ways a real parser wouldn't. I'd
either swap in `golang.org/x/net/html` and rewrite the extraction as a
tree walk, or at minimum add a pre-pass that strips HTML comments before
the regexes run, which is the single most common false-positive source
I hit while testing against real sites.

## Project layout

```
.
├── main.go          # HTTP server, routing, error → status mapping
├── audit.go         # Fetch + parse logic (the testable core)
├── audit_test.go    # Happy path + 4 failure-case tests
├── static/
│   └── index.html   # Frontend: input field + rendered report
└── README.md
```
