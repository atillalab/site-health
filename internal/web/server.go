package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/atillalab/site-health/internal/check"
	"github.com/atillalab/site-health/internal/domain"
)

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	http.HandleFunc("/api/projects", handleProjects)
	http.HandleFunc("/api/projects/", handleProjectDetail)
	http.HandleFunc("/api/active-project", handleActiveProject)

	addr := "localhost:" + port
	fmt.Printf("Starting site-health web on http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop.")
	return http.ListenAndServe(addr, nil)
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		idx, err := loadProjectsIndex()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, idx)

	case http.MethodPost:
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
		p, err := createProject(req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(rest, "/", 2)
	projectID := parts[0]
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
			http.Error(w, err.Error(), http.StatusNotFound)
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
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, p)

	case http.MethodDelete:
		if err := deleteProject(projectID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleActiveProject(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p, err := getActiveProject()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, err := getProject(req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectDomains(w http.ResponseWriter, r *http.Request, projectID string) {
	if _, err := getProject(projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		store, err := loadDomains(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, domainStore{Domains: cleaned})

	case http.MethodDelete:
		if err := clearDomains(projectID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, domainStore{Domains: []string{}})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectSettings(w http.ResponseWriter, r *http.Request, projectID string) {
	if _, err := getProject(projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := loadProjectSettings(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, settings)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectSessions(w http.ResponseWriter, r *http.Request, projectID string) {
	if _, err := getProject(projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		summaries, err := listSessionSummaries(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sessions": summaries})

	case http.MethodDelete:
		if err := clearSessions(projectID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sessions": []SessionSummary{}})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleProjectCheck(w http.ResponseWriter, r *http.Request, projectID string) {
	if _, err := getProject(projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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

func handleProjectSession(w http.ResponseWriter, r *http.Request, projectID, sessionID string) {
	if _, err := getProject(projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	var wg sync.WaitGroup
	results := make([]*check.Report, len(domains))
	for i, d := range domains {
		wg.Add(1)
		go func(idx int, raw string) {
			defer wg.Done()
			normalized := domain.Normalize(raw)
			if normalized == "" {
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
			}
			results[idx] = runner.RunChecks(ctx)
		}(i, d)
	}
	wg.Wait()
	return results
}
