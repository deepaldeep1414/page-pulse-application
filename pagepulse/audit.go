package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Report is the JSON shape returned by the /api/audit endpoint.
type Report struct {
	URL              string `json:"url"`
	StatusCode       int    `json:"statusCode"`
	ResponseTimeMs   int64  `json:"responseTimeMs"`
	Title            string `json:"title"`
	MetaDescription  string `json:"metaDescription"`
	H1Count          int    `json:"h1Count"`
	ImagesTotal      int    `json:"imagesTotal"`
	ImagesMissingAlt int    `json:"imagesMissingAlt"`
	WordCount        int    `json:"wordCount"`
}

// AuditError is a typed error so the HTTP layer can pick the right status
// code and message instead of leaking raw Go errors to the client.
type AuditError struct {
	// Kind classifies the failure: "invalid_url", "timeout", "unreachable",
	// "non_html", or "upstream_error".
	Kind    string
	Message string
	Cause   error
}

func (e *AuditError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AuditError) Unwrap() error { return e.Cause }

var (
	titleRe       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaDescRe    = regexp.MustCompile(`(?is)<meta\s+[^>]*name=["']description["'][^>]*>`)
	metaContentRe = regexp.MustCompile(`(?is)content=["'](.*?)["']`)
	h1Re          = regexp.MustCompile(`(?is)<h1[^>]*>`)
	imgRe         = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	altRe         = regexp.MustCompile(`(?is)\balt\s*=\s*["'](.*?)["']`)
	tagRe         = regexp.MustCompile(`(?is)<script.*?</script>|<style.*?</style>|<[^>]+>`)
	spaceRe       = regexp.MustCompile(`\s+`)
)

// validateURL enforces that the input is an absolute http(s) URL. This is a
// deliberate, narrow check: we would rather reject a borderline input with a
// clear error than pass something ambiguous to net/http and get a confusing
// failure two layers down.
func validateURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, &AuditError{Kind: "invalid_url", Message: "url is required"}
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, &AuditError{Kind: "invalid_url", Message: "url is not a valid absolute URL", Cause: err}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, &AuditError{Kind: "invalid_url", Message: "url must use http or https"}
	}
	if u.Host == "" {
		return nil, &AuditError{Kind: "invalid_url", Message: "url is missing a host"}
	}
	return u, nil
}

// FetchAndAudit fetches rawURL with the given client and produces a Report.
// It never panics: any failure is returned as an *AuditError.
func FetchAndAudit(rawURL string, client *http.Client) (*Report, error) {
	u, err := validateURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, &AuditError{Kind: "invalid_url", Message: "could not build request", Cause: err}
	}
	// A normal-looking UA avoids sites that reject requests with no UA at all.
	req.Header.Set("User-Agent", "PagePulse/1.0 (+https://digitalheroesco.com)")

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		if isTimeout(err) {
			return nil, &AuditError{Kind: "timeout", Message: "the request timed out", Cause: err}
		}
		return nil, &AuditError{Kind: "unreachable", Message: "could not reach that URL", Cause: err}
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil, &AuditError{
			Kind:    "non_html",
			Message: fmt.Sprintf("expected an HTML page but got content-type %q", contentType),
		}
	}

	// Cap the read so a huge or endless response can't exhaust memory.
	const maxBody = 5 << 20 // 5MB
	limited := io.LimitReader(resp.Body, maxBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, &AuditError{Kind: "upstream_error", Message: "failed reading response body", Cause: err}
	}
	if len(body) > maxBody {
		return nil, &AuditError{Kind: "upstream_error", Message: "response body exceeded 5MB limit"}
	}

	html := string(body)

	report := &Report{
		URL:             u.String(),
		StatusCode:      resp.StatusCode,
		ResponseTimeMs:  elapsed.Milliseconds(),
		Title:           extractTitle(html),
		MetaDescription: extractMetaDescription(html),
		H1Count:         len(h1Re.FindAllString(html, -1)),
	}

	imagesTotal, imagesMissingAlt := countImageAlt(html)
	report.ImagesTotal = imagesTotal
	report.ImagesMissingAlt = imagesMissingAlt
	report.WordCount = countWords(html)

	return report, nil
}

func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")
}

func extractTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return decodeAndClean(m[1])
}

func extractMetaDescription(html string) string {
	tag := metaDescRe.FindString(html)
	if tag == "" {
		return ""
	}
	m := metaContentRe.FindStringSubmatch(tag)
	if len(m) < 2 {
		return ""
	}
	return decodeAndClean(m[1])
}

func countImageAlt(html string) (total int, missing int) {
	imgs := imgRe.FindAllString(html, -1)
	total = len(imgs)
	for _, tag := range imgs {
		m := altRe.FindStringSubmatch(tag)
		if len(m) < 2 || strings.TrimSpace(m[1]) == "" {
			missing++
		}
	}
	return total, missing
}

// countWords strips tags/scripts/styles and counts remaining whitespace-
// separated tokens. This is an approximation, not a typographic word count:
// it will over-count on pages with lots of inline attributes rendered as
// text-like content, and under-count content injected purely by JS.
func countWords(html string) int {
	stripped := tagRe.ReplaceAllString(html, " ")
	stripped = decodeEntities(stripped)
	stripped = strings.TrimSpace(spaceRe.ReplaceAllString(stripped, " "))
	if stripped == "" {
		return 0
	}
	return len(strings.Split(stripped, " "))
}

func decodeAndClean(s string) string {
	s = decodeEntities(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

var entityRe = regexp.MustCompile(`&(#\d+|#x[0-9a-fA-F]+|[a-zA-Z]+);`)

// decodeEntities handles the common named/numeric HTML entities. We avoid a
// full HTML entity table on purpose -- see README design decision #3.
func decodeEntities(s string) string {
	return entityRe.ReplaceAllStringFunc(s, func(ent string) string {
		inner := ent[1 : len(ent)-1]
		switch inner {
		case "amp":
			return "&"
		case "lt":
			return "<"
		case "gt":
			return ">"
		case "quot":
			return `"`
		case "apos", "#39":
			return "'"
		case "nbsp":
			return " "
		}
		if strings.HasPrefix(inner, "#x") || strings.HasPrefix(inner, "#X") {
			if n, err := strconv.ParseInt(inner[2:], 16, 32); err == nil {
				return string(rune(n))
			}
		} else if strings.HasPrefix(inner, "#") {
			if n, err := strconv.ParseInt(inner[1:], 10, 32); err == nil {
				return string(rune(n))
			}
		}
		return ent
	})
}
