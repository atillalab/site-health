package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atillalab/site-health/internal/check"
)

// Session represents a single check run for a set of domains within a project.
type Session struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Domains   []string        `json:"domains"`
	Results   []*check.Report `json:"results"`
}

// dataDirOverride is used by tests to redirect all web data to a temporary directory.
var dataDirOverride string

// dataDir returns the directory used for web UI data and creates it if needed.
func dataDir() (string, error) {
	if dataDirOverride != "" {
		if err := os.MkdirAll(dataDirOverride, 0o700); err != nil {
			return "", err
		}
		if err := os.Chmod(dataDirOverride, 0o700); err != nil {
			return "", err
		}
		return dataDirOverride, nil
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, "site-health", "web")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// projectDataDir returns the directory used for a project's data.
func projectDataDir(projectID string) string {
	return projectDir(projectID)
}

// projectSessionsDir returns the directory used for a project's session files.
func projectSessionsDir(projectID string) string {
	return filepath.Join(projectDataDir(projectID), "sessions")
}

// projectDomainsPath returns the path to a project's persisted domain list.
func projectDomainsPath(projectID string) string {
	return filepath.Join(projectDataDir(projectID), "domains.json")
}

// domainStore is the on-disk format for the domain list.
type domainStore struct {
	Domains []string `json:"domains"`
}

// loadDomains reads the saved domain list for a project. A missing file is
// treated as empty.
func loadDomains(projectID string) (*domainStore, error) {
	path := projectDomainsPath(projectID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domainStore{Domains: []string{}}, nil
		}
		return nil, err
	}
	var store domainStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// saveDomains writes the domain list for a project to disk.
func saveDomains(projectID string, domains []string) error {
	path := projectDomainsPath(projectID)
	store := domainStore{Domains: domains}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// clearDomains removes the persisted domain list for a project.
func clearDomains(projectID string) error {
	path := projectDomainsPath(projectID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// newSessionID returns a filename-safe timestamp-based ID.
func newSessionID(t time.Time) string {
	return t.Format("2006-01-02T15-04-05")
}

// saveSession persists a check run for a project and returns the created session.
func saveSession(projectID string, domains []string, results []*check.Report) (*Session, error) {
	dir := projectSessionsDir(projectID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	now := time.Now()
	session := &Session{
		ID:        newSessionID(now),
		Timestamp: now,
		Domains:   domains,
		Results:   results,
	}
	path := filepath.Join(dir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return session, nil
}

// IssueSummary is a lightweight issue entry for a single domain in a session.
type IssueSummary struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ResultSummary is a lightweight status entry for a single domain in a session.
type ResultSummary struct {
	Domain string         `json:"domain"`
	Status string         `json:"status"`
	Issues []IssueSummary `json:"issues"`
}

// SessionSummary is a lightweight view of a session for history listings.
type SessionSummary struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Domains   []string        `json:"domains"`
	Results   []ResultSummary `json:"results"`
}

// listSessionSummaries returns a summary of each session for a project, sorted newest first.
func listSessionSummaries(projectID string) ([]*SessionSummary, error) {
	dir := projectSessionsDir(projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	summaries := []*SessionSummary{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		session, err := loadSession(projectID, id)
		if err != nil {
			return nil, err
		}
		results := make([]ResultSummary, 0, len(session.Results))
		for _, r := range session.Results {
			status := r.Summary.Status
			if status == "" {
				status = "UNKNOWN"
			}
			issues := make([]IssueSummary, 0, len(r.Issues))
			for _, issue := range r.Issues {
				issues = append(issues, IssueSummary{
					Level:   issue.Level,
					Message: issue.Message,
				})
			}
			results = append(results, ResultSummary{
				Domain: r.Domain,
				Status: status,
				Issues: issues,
			})
		}
		summaries = append(summaries, &SessionSummary{
			ID:        session.ID,
			Timestamp: session.Timestamp,
			Domains:   session.Domains,
			Results:   results,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Timestamp.After(summaries[j].Timestamp)
	})
	return summaries, nil
}

// loadSession reads a session file by project ID and session ID.
func loadSession(projectID, sessionID string) (*Session, error) {
	if !projectIDRe.MatchString(projectID) || !sessionIDRe.MatchString(sessionID) {
		return nil, os.ErrInvalid
	}
	path := filepath.Join(projectSessionsDir(projectID), sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// clearSessions removes all saved session files for a project.
func clearSessions(projectID string) error {
	dir := projectSessionsDir(projectID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
