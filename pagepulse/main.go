package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed static/index.html
var staticFiles embed.FS

// httpClient is shared across requests. The timeout is the single place
// that bounds how long a slow/hanging target site can tie up a request.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

type errorResponse struct {
	Error string `json:"error"`
	Kind  string `json:"kind"`
}

func main() {
	mux := http.NewServeMux()

	indexBytes, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexBytes)
	})

	mux.HandleFunc("/api/audit", auditHandler)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	handler := recoverMiddleware(mux)

	log.Printf("Page Pulse listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

// auditHandler accepts GET (?url=) or POST ({"url": "..."}) and always
// responds with JSON, even on failure -- callers never have to sniff the
// body to figure out whether they got an error.
func auditHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var target string
	switch r.Method {
	case http.MethodGet:
		target = r.URL.Query().Get("url")
	case http.MethodPost:
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, &AuditError{Kind: "invalid_request", Message: "body must be JSON with a \"url\" field"})
			return
		}
		target = body.URL
	default:
		writeError(w, http.StatusMethodNotAllowed, &AuditError{Kind: "method_not_allowed", Message: "use GET ?url= or POST {\"url\":...}"})
		return
	}

	report, err := FetchAndAudit(target, httpClient)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}

	json.NewEncoder(w).Encode(report)
}

func statusForError(err error) int {
	ae, ok := err.(*AuditError)
	if !ok {
		return http.StatusInternalServerError
	}
	switch ae.Kind {
	case "invalid_url", "invalid_request":
		return http.StatusBadRequest
	case "non_html":
		return http.StatusUnprocessableEntity
	case "timeout":
		return http.StatusGatewayTimeout
	case "unreachable":
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	kind := "internal_error"
	if ae, ok := err.(*AuditError); ok {
		kind = ae.Kind
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: err.Error(), Kind: kind})
}

// recoverMiddleware guarantees that a bug in parsing (a bad regex match, a
// nil deref, etc.) turns into a 500 JSON response instead of taking the
// whole process down. This is the backstop behind the explicit error
// handling in audit.go, not a substitute for it.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("recovered panic: %v", rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(errorResponse{Error: "internal server error", Kind: "internal_error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
