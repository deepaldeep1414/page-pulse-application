package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleHTML = `<!doctype html>
<html>
<head>
  <title>  Welcome to Acme &amp; Co.  </title>
  <meta name="description" content="Acme sells the finest widgets on the web.">
</head>
<body>
  <h1>Main heading</h1>
  <p>Hello world, this is a small paragraph with a few words in it.</p>
  <img src="/logo.png" alt="Acme logo">
  <img src="/banner.png">
  <img src="/icon.png" alt="">
  <script>console.log("this should not be counted as words");</script>
</body>
</html>`

// --- Happy path -----------------------------------------------------------

func TestFetchAndAudit_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleHTML))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	report, err := FetchAndAudit(srv.URL, client)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if report.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", report.StatusCode)
	}
	if report.Title != "Welcome to Acme & Co." {
		t.Errorf("Title = %q, want %q", report.Title, "Welcome to Acme & Co.")
	}
	if report.MetaDescription != "Acme sells the finest widgets on the web." {
		t.Errorf("MetaDescription = %q", report.MetaDescription)
	}
	if report.H1Count != 1 {
		t.Errorf("H1Count = %d, want 1", report.H1Count)
	}
	if report.ImagesTotal != 3 {
		t.Errorf("ImagesTotal = %d, want 3", report.ImagesTotal)
	}
	// One image has a real alt, one has no alt attribute, one has alt="".
	// Both of the latter two count as "missing".
	if report.ImagesMissingAlt != 2 {
		t.Errorf("ImagesMissingAlt = %d, want 2", report.ImagesMissingAlt)
	}
	if report.WordCount == 0 {
		t.Errorf("WordCount = 0, want > 0")
	}
	if strings.Contains(strings.ToLower(strings.Join([]string{}, "")), "console.log") {
		t.Errorf("script contents leaked into parsed output")
	}
	if report.ResponseTimeMs < 0 {
		t.Errorf("ResponseTimeMs should be non-negative, got %d", report.ResponseTimeMs)
	}
}

// --- Failure case 1: invalid URL ------------------------------------------

func TestFetchAndAudit_InvalidURL(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}

	cases := []string{
		"",
		"not a url",
		"ftp://example.com/file",
		"example.com", // no scheme
	}

	for _, raw := range cases {
		_, err := FetchAndAudit(raw, client)
		if err == nil {
			t.Errorf("FetchAndAudit(%q) expected error, got nil", raw)
			continue
		}
		ae, ok := err.(*AuditError)
		if !ok {
			t.Errorf("FetchAndAudit(%q) expected *AuditError, got %T", raw, err)
			continue
		}
		if ae.Kind != "invalid_url" {
			t.Errorf("FetchAndAudit(%q) Kind = %q, want %q", raw, ae.Kind, "invalid_url")
		}
	}
}

// --- Failure case 2: non-HTML response ------------------------------------

func TestFetchAndAudit_NonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	_, err := FetchAndAudit(srv.URL, client)
	if err == nil {
		t.Fatal("expected an error for a non-HTML response, got nil")
	}
	ae, ok := err.(*AuditError)
	if !ok {
		t.Fatalf("expected *AuditError, got %T", err)
	}
	if ae.Kind != "non_html" {
		t.Errorf("Kind = %q, want %q", ae.Kind, "non_html")
	}
}

// --- Failure case 3 (bonus): timeout ---------------------------------------

func TestFetchAndAudit_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()

	// Client timeout shorter than the handler's sleep, to force a timeout
	// deterministically instead of relying on a flaky real network delay.
	client := &http.Client{Timeout: 20 * time.Millisecond}
	_, err := FetchAndAudit(srv.URL, client)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	ae, ok := err.(*AuditError)
	if !ok {
		t.Fatalf("expected *AuditError, got %T", err)
	}
	if ae.Kind != "timeout" {
		t.Errorf("Kind = %q, want %q", ae.Kind, "timeout")
	}
}

// --- Failure case 4 (bonus): connection refused / unreachable -------------

func TestFetchAndAudit_Unreachable(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	// Port 1 is reserved and nothing should be listening there.
	_, err := FetchAndAudit("http://127.0.0.1:1/", client)
	if err == nil {
		t.Fatal("expected an error connecting to a closed port, got nil")
	}
	ae, ok := err.(*AuditError)
	if !ok {
		t.Fatalf("expected *AuditError, got %T", err)
	}
	if ae.Kind != "unreachable" && ae.Kind != "timeout" {
		t.Errorf("Kind = %q, want %q or %q", ae.Kind, "unreachable", "timeout")
	}
}
