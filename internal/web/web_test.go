package web

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atillalab/site-health/internal/check"
)

func setupTestDataDir(t *testing.T) func() {
	t.Helper()
	dir, err := os.MkdirTemp("", "site-health-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	oldOverride := dataDirOverride
	dataDirOverride = dir
	return func() {
		dataDirOverride = oldOverride
		os.RemoveAll(dir)
	}
}

func TestCreateRenameDeleteProject(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()

	p, err := createProject("Example")
	if err != nil {
		t.Fatalf("createProject failed: %v", err)
	}
	if p.Name != "Example" {
		t.Errorf("expected name Example, got %q", p.Name)
	}

	idx, err := loadProjectsIndex()
	if err != nil {
		t.Fatalf("loadProjectsIndex failed: %v", err)
	}
	if len(idx.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(idx.Projects))
	}

	renamed, err := renameProject(p.ID, "Renamed")
	if err != nil {
		t.Fatalf("renameProject failed: %v", err)
	}
	if renamed.Name != "Renamed" {
		t.Errorf("expected name Renamed, got %q", renamed.Name)
	}

	if err := deleteProject(p.ID); err != nil {
		t.Fatalf("deleteProject failed: %v", err)
	}
	idx, err = loadProjectsIndex()
	if err != nil {
		t.Fatalf("loadProjectsIndex after delete failed: %v", err)
	}
	if len(idx.Projects) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(idx.Projects))
	}
}

func TestActiveProject(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()

	p, err := createProject("Active Test")
	if err != nil {
		t.Fatalf("createProject failed: %v", err)
	}

	if err := setActiveProject(p.ID); err != nil {
		t.Fatalf("setActiveProject failed: %v", err)
	}

	active, err := getActiveProject()
	if err != nil {
		t.Fatalf("getActiveProject failed: %v", err)
	}
	if active == nil || active.ID != p.ID {
		t.Errorf("expected active project %s, got %v", p.ID, active)
	}
}

func TestSaveLoadListClearSessions(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()

	p, err := createProject("Session Test")
	if err != nil {
		t.Fatalf("createProject failed: %v", err)
	}

	domains := []string{"example.com"}
	results := []*check.Report{
		{
			Domain: "example.com",
			Summary: check.Summary{
				Status:   "HEALTHY",
				Failures: 0,
				Warnings: 0,
			},
		},
	}

	session, err := saveSession(p.ID, domains, results)
	if err != nil {
		t.Fatalf("saveSession failed: %v", err)
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if len(session.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(session.Results))
	}

	loaded, err := loadSession(p.ID, session.ID)
	if err != nil {
		t.Fatalf("loadSession failed: %v", err)
	}
	if len(loaded.Domains) != len(session.Domains) || loaded.Domains[0] != session.Domains[0] {
		t.Errorf("expected domains %v, got %v", session.Domains, loaded.Domains)
	}

	summaries, err := listSessionSummaries(p.ID)
	if err != nil {
		t.Fatalf("listSessionSummaries failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].ID != session.ID {
		t.Errorf("expected summary ID %s, got %s", session.ID, summaries[0].ID)
	}
	if len(summaries[0].Results) != 1 {
		t.Errorf("expected 1 result summary, got %d", len(summaries[0].Results))
	}

	if err := clearSessions(p.ID); err != nil {
		t.Fatalf("clearSessions failed: %v", err)
	}
	summaries, err = listSessionSummaries(p.ID)
	if err != nil {
		t.Fatalf("listSessionSummaries after clear failed: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries after clear, got %d", len(summaries))
	}

	sessionPath := filepath.Join(projectSessionsDir(p.ID), session.ID+".json")
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Errorf("session file should have been removed: %s", sessionPath)
	}
}

func TestProjectSettings(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()

	p, err := createProject("Settings Test")
	if err != nil {
		t.Fatalf("createProject failed: %v", err)
	}

	settings, err := loadProjectSettings(p.ID)
	if err != nil {
		t.Fatalf("loadProjectSettings failed: %v", err)
	}
	if settings.SkipRedirect {
		t.Error("expected SkipRedirect to default to false")
	}

	settings.SkipRedirect = true
	if err := saveProjectSettings(p.ID, settings); err != nil {
		t.Fatalf("saveProjectSettings failed: %v", err)
	}

	loaded, err := loadProjectSettings(p.ID)
	if err != nil {
		t.Fatalf("loadProjectSettings after save failed: %v", err)
	}
	if !loaded.SkipRedirect {
		t.Error("expected SkipRedirect to be true after saving")
	}
}

func TestListSessionSummariesOrdering(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()

	p, err := createProject("Ordering Test")
	if err != nil {
		t.Fatalf("createProject failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := saveSession(p.ID, []string{"example.com"}, []*check.Report{
			{Domain: "example.com", Summary: check.Summary{Status: "HEALTHY"}},
		})
		if err != nil {
			t.Fatalf("saveSession failed: %v", err)
		}
		time.Sleep(1100 * time.Millisecond)
	}

	summaries, err := listSessionSummaries(p.ID)
	if err != nil {
		t.Fatalf("listSessionSummaries failed: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}
	for i := 1; i < len(summaries); i++ {
		if !summaries[i-1].Timestamp.After(summaries[i].Timestamp) {
			t.Errorf("summaries not sorted newest first at index %d", i)
		}
	}
}
