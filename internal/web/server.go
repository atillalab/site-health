package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/atillalab/site-health/internal/check"
)

const (
	maxRequestBody      = 1 << 20
	maxDomainsPerCheck  = 50
	maxConcurrentChecks = 8
)

var sessionIDRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}$`)

//go:embed index.html
var indexHTML []byte

// Run starts the local web server for site-health.
// It accepts a --port flag in args (default 8080).
func Run(args []string) error {
	port := "8080"
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			port = args[i+1]
			i++
		}
	}

	if err := migrateLegacyData(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	mux.HandleFunc("/api/projects", handleProjects)
	mux.HandleFunc("/api/projects/", handleProjectDetail)
	mux.HandleFunc("/api/active-project", handleActiveProject)

	addr := "localhost:" + port
	fmt.Printf("Starting site-health web on http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop.")
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	return server.ListenAndServe()
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")
}

func publicError(error) string { return "internal server error" }

func protectRequest(w http.ResponseWriter, r *http.Request) bool {
	setSecurityHeaders(w)
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			localOrigin := err == nil && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
			localHost := r.Host == "" || strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") || strings.HasPrefix(r.Host, "[::1]")
			if !localOrigin || !localHost {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return false
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	}
	return true
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	if !protectRequest(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		idx, err := loadProjectsIndex()
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, idx)

	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if _, ok := err.(*http.MaxBytesError); ok {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			http.Error(w, "project name is required", http.StatusBadRequest)
			return
		}
		p, err := createProject(req.Name)
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	if !protectRequest(w, r) {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(rest, "/", 2)
	projectID := parts[0]
	if !projectIDRe.MatchString(projectID) {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
		return
	}
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	if sub == "" {
		handleProjectResource(w, r, projectID)
		return
	}

	switch sub {
	case "domains":
		handleProjectDomains(w, r, projectID)
	case "sessions":
		handleProjectSessions(w, r, projectID)
	case "check":
		handleProjectCheck(w, r, projectID)
	case "settings":
		handleProjectSettings(w, r, projectID)
	default:
		if strings.HasPrefix(sub, "sessions/") {
			sessionID := strings.TrimPrefix(sub, "sessions/")
			handleProjectSession(w, r, projectID, sessionID)
			return
		}
		http.NotFound(w, r)
	}
}

func handleProjectResource(w http.ResponseWriter, r *http.Request, projectID string) {
	switch r.Method {
	case http.MethodGet:
		p, err := getProject(projectID)
		if err != nil {
			http.Error(w, publicError(err), http.StatusNotFound)
			return
		}
		writeJSON(w, p)

	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			http.Error(w, "project name is required", http.StatusBadRequest)
			return
		}
		p, err := renameProject(projectID, req.Name)
		if err != nil {
			http.Error(w, publicError(err), http.StatusNotFound)
			return
		}
		writeJSON(w, p)

	case http.MethodDelete:
		if err := deleteProject(projectID); err != nil {
			http.Error(w, publicError(err), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleActiveProject(w http.ResponseWriter, r *http.Request) {
	if !protectRequest(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := getActiveProject()
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		if p == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, p)

	case http.MethodPut:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := setActiveProject(req.ID); err != nil {
			http.Error(w, publicError(err), http.StatusBadRequest)
			return
		}
		p, err := getProject(req.ID)
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectDomains(w http.ResponseWriter, r *http.Request, projectID string) {
	if !protectRequest(w, r) {
		return
	}
	if _, err := getProject(projectID); err != nil {
		http.Error(w, publicError(err), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		store, err := loadDomains(projectID)
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, store)

	case http.MethodPost:
		var req domainStore
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		cleaned := cleanDomains(req.Domains)
		if err := saveDomains(projectID, cleaned); err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, domainStore{Domains: cleaned})

	case http.MethodDelete:
		if err := clearDomains(projectID); err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, domainStore{Domains: []string{}})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectSettings(w http.ResponseWriter, r *http.Request, projectID string) {
	if !protectRequest(w, r) {
		return
	}
	if _, err := getProject(projectID); err != nil {
		http.Error(w, publicError(err), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := loadProjectSettings(projectID)
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, settings)

	case http.MethodPut:
		var settings ProjectSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := saveProjectSettings(projectID, &settings); err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, settings)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectSessions(w http.ResponseWriter, r *http.Request, projectID string) {
	if !protectRequest(w, r) {
		return
	}
	if _, err := getProject(projectID); err != nil {
		http.Error(w, publicError(err), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		summaries, err := listSessionSummaries(projectID)
		if err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sessions": summaries})

	case http.MethodDelete:
		if err := clearSessions(projectID); err != nil {
			http.Error(w, publicError(err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sessions": []SessionSummary{}})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectCheck(w http.ResponseWriter, r *http.Request, projectID string) {
	if !protectRequest(w, r) {
		return
	}
	if _, err := getProject(projectID); err != nil {
		http.Error(w, publicError(err), http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domains      []string `json:"domains"`
		SkipRedirect bool     `json:"skip_redirect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	domains := cleanDomains(req.Domains)
	if err := validateDomainCount(domains); err != nil {
		http.Error(w, "too many domains", http.StatusRequestEntityTooLarge)
		return
	}
	if len(domains) == 0 {
		http.Error(w, "no domains to check", http.StatusBadRequest)
		return
	}

	results := runChecksForDomains(r.Context(), domains, req.SkipRedirect)
	session, err := saveSession(projectID, domains, results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to save session: %v\n", err)
	}
	resp := map[string]any{"results": results}
	if session != nil {
		resp["session_id"] = session.ID
	}
	writeJSON(w, resp)
}

func validateDomainCount(domains []string) error {
	if len(domains) > maxDomainsPerCheck {
		return fmt.Errorf("too many domains")
	}
	return nil
}

func handleProjectSession(w http.ResponseWriter, r *http.Request, projectID, sessionID string) {
	if !protectRequest(w, r) {
		return
	}
	if !sessionIDRe.MatchString(sessionID) {
		http.Error(w, "invalid session ID", http.StatusBadRequest)
		return
	}
	if _, err := getProject(projectID); err != nil {
		http.Error(w, publicError(err), http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, err := loadSession(projectID, sessionID)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		http.Error(w, publicError(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, session)
}

// cleanDomains trims whitespace, removes empty entries, and deduplicates.
func cleanDomains(domains []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		if _, err := validateWebDomain(d); err != nil {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// runChecksForDomains runs site checks for the given domains in parallel and
// returns a report for each one, preserving the input order.
func runChecksForDomains(ctx context.Context, domains []string, skipRedirect bool) []*check.Report {
	results := make([]*check.Report, len(domains))
	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup
	for i, d := range domains {
		wg.Add(1)
		go func(idx int, raw string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			normalized, err := publicWebDomain(raw)
			if err != nil || normalized == "" {
				results[idx] = &check.Report{
					Domain: raw,
					Mode:   "site",
					Summary: check.Summary{
						Status:   "UNHEALTHY",
						Failures: 1,
					},
					Issues: []check.Issue{{Level: "FAIL", Message: "invalid domain"}},
				}
				return
			}
			runner := &check.Runner{
				Domain:        normalized,
				ExpectedHosts: []string{normalized},
				MailChecks:    check.DefaultMailChecks(),
				SkipRedirect:  skipRedirect,
				PublicOnly:    true,
			}
			results[idx] = runDomainCheck(ctx, runner)
		}(i, d)
	}
	wg.Wait()
	return results
}

var runDomainCheck = func(ctx context.Context, runner *check.Runner) *check.Report { return runner.RunChecks(ctx) }
