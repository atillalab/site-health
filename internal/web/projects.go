package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultProjectID = "default"

// Project represents a folder that groups domains and their check sessions.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectsIndex is the top-level registry of projects and the active project.
type ProjectsIndex struct {
	Projects      []Project `json:"projects"`
	ActiveProject string    `json:"active_project"`
}

var projectIDRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

// projectsIndexPath returns the path to the projects registry file.
func projectsIndexPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projects.json"), nil
}

// loadProjectsIndex reads the projects registry. A missing file returns an empty
// registry.
func loadProjectsIndex() (*ProjectsIndex, error) {
	path, err := projectsIndexPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectsIndex{Projects: []Project{}}, nil
		}
		return nil, err
	}
	var idx ProjectsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// saveProjectsIndex writes the projects registry to disk.
func saveProjectsIndex(idx *ProjectsIndex) error {
	path, err := projectsIndexPath()
	if err != nil {
		return err
	}
	sort.Slice(idx.Projects, func(i, j int) bool {
		return idx.Projects[i].CreatedAt.Before(idx.Projects[j].CreatedAt)
	})
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// slugifyName converts a project name to a URL-safe base ID. It transliterates
// common non-ASCII characters before stripping the rest.
func slugifyName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	replacements := map[string]string{
		"ç": "c", "ğ": "g", "ı": "i", "ö": "o", "ş": "s", "ü": "u",
		"â": "a", "ê": "e", "î": "i", "ô": "o", "û": "u",
	}
	for old, new := range replacements {
		s = strings.ReplaceAll(s, old, new)
	}
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "project"
	}
	return s
}

// makeProjectID returns a unique project ID based on the given name.
func makeProjectID(name string, idx *ProjectsIndex) string {
	base := slugifyName(name)
	id := base
	counter := 1
	for projectExists(id, idx) {
		id = fmt.Sprintf("%s-%d", base, counter)
		counter++
	}
	return id
}

// projectExists reports whether a project with the given ID is registered.
func projectExists(id string, idx *ProjectsIndex) bool {
	for _, p := range idx.Projects {
		if p.ID == id {
			return true
		}
	}
	return false
}

// createProject creates a new project with the given name and returns it.
func createProject(name string) (*Project, error) {
	idx, err := loadProjectsIndex()
	if err != nil {
		return nil, err
	}
	id := makeProjectID(name, idx)
	p := Project{
		ID:        id,
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now(),
	}
	idx.Projects = append(idx.Projects, p)
	if idx.ActiveProject == "" {
		idx.ActiveProject = id
	}
	if err := saveProjectsIndex(idx); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(projectDir(id), "sessions"), 0o700); err != nil {
		return nil, err
	}
	if err := saveDomains(id, []string{}); err != nil {
		return nil, err
	}
	return &p, nil
}

// deleteProject removes a project and all its data. If the deleted project was
// the active one, the next remaining project becomes active (or none if it was
// the last project).
func deleteProject(id string) error {
	idx, err := loadProjectsIndex()
	if err != nil {
		return err
	}
	var found bool
	var updated []Project
	for _, p := range idx.Projects {
		if p.ID == id {
			found = true
			continue
		}
		updated = append(updated, p)
	}
	if !found {
		return fmt.Errorf("project not found")
	}
	idx.Projects = updated
	if idx.ActiveProject == id {
		if len(updated) > 0 {
			idx.ActiveProject = updated[0].ID
		} else {
			idx.ActiveProject = ""
		}
	}
	if err := saveProjectsIndex(idx); err != nil {
		return err
	}
	return os.RemoveAll(projectDir(id))
}

// renameProject updates a project's display name.
func renameProject(id, name string) (*Project, error) {
	idx, err := loadProjectsIndex()
	if err != nil {
		return nil, err
	}
	for i := range idx.Projects {
		if idx.Projects[i].ID == id {
			idx.Projects[i].Name = strings.TrimSpace(name)
			if err := saveProjectsIndex(idx); err != nil {
				return nil, err
			}
			return &idx.Projects[i], nil
		}
	}
	return nil, fmt.Errorf("project not found")
}

// setActiveProject updates the active project in the registry.
func setActiveProject(id string) error {
	idx, err := loadProjectsIndex()
	if err != nil {
		return err
	}
	if !projectExists(id, idx) {
		return fmt.Errorf("project not found")
	}
	idx.ActiveProject = id
	return saveProjectsIndex(idx)
}

// getActiveProject returns the currently active project, or nil if no projects
// exist yet.
func getActiveProject() (*Project, error) {
	idx, err := loadProjectsIndex()
	if err != nil {
		return nil, err
	}
	if len(idx.Projects) == 0 {
		return nil, nil
	}
	for _, p := range idx.Projects {
		if p.ID == idx.ActiveProject {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("active project not found")
}

// getProject returns a project by ID.
func getProject(id string) (*Project, error) {
	idx, err := loadProjectsIndex()
	if err != nil {
		return nil, err
	}
	for _, p := range idx.Projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project not found")
}

// projectDir returns the directory used for a project's data.
func projectDir(id string) string {
	dir, _ := dataDir()
	if !projectIDRe.MatchString(id) {
		return filepath.Join(dir, "projects", "invalid")
	}
	return filepath.Join(dir, "projects", id)
}

// ProjectSettings holds per-project configuration.
type ProjectSettings struct {
	SkipRedirect bool `json:"skip_redirect"`
}

// projectSettingsPath returns the path to a project's settings file.
func projectSettingsPath(projectID string) string {
	return filepath.Join(projectDir(projectID), "settings.json")
}

// loadProjectSettings reads a project's settings. A missing file returns defaults.
func loadProjectSettings(projectID string) (*ProjectSettings, error) {
	path := projectSettingsPath(projectID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectSettings{}, nil
		}
		return nil, err
	}
	var settings ProjectSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// saveProjectSettings writes a project's settings to disk.
func saveProjectSettings(projectID string, settings *ProjectSettings) error {
	path := projectSettingsPath(projectID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ensureDefaultProject creates a default project if no projects exist yet.
func ensureDefaultProject() error {
	idx, err := loadProjectsIndex()
	if err != nil {
		return err
	}
	if len(idx.Projects) == 0 {
		_, err := createProject("Default Project")
		if err != nil {
			return err
		}
		// Rename the created project so its ID is exactly "default".
		idx, err = loadProjectsIndex()
		if err != nil {
			return err
		}
		for i := range idx.Projects {
			if idx.Projects[i].Name == "Default Project" && idx.Projects[i].ID != defaultProjectID {
				oldID := idx.Projects[i].ID
				idx.Projects[i].ID = defaultProjectID
				if err := saveProjectsIndex(idx); err != nil {
					return err
				}
				oldDir := projectDir(oldID)
				newDir := projectDir(defaultProjectID)
				if err := os.MkdirAll(filepath.Dir(newDir), 0o700); err != nil {
					return err
				}
				if err := os.Rename(oldDir, newDir); err != nil {
					return err
				}
				break
			}
		}
		idx.ActiveProject = defaultProjectID
		return saveProjectsIndex(idx)
	}
	return nil
}

// migrateLegacyData moves old flat domains.json and sessions/ into the default
// project. It should be called once when the web server starts. Empty legacy
// data is ignored so that a truly fresh start does not create a default project.
func migrateLegacyData() error {
	dir, err := dataDir()
	if err != nil {
		return err
	}
	legacyDomains := filepath.Join(dir, "domains.json")
	legacySessions := filepath.Join(dir, "sessions")

	hasDomains := legacyDomainsHasData(legacyDomains)
	hasSessions := legacySessionsHasData(legacySessions)

	if !hasDomains && !hasSessions {
		return nil
	}

	if err := ensureDefaultProject(); err != nil {
		return err
	}

	if hasDomains {
		target := filepath.Join(projectDir(defaultProjectID), "domains.json")
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(legacyDomains, target); err != nil {
			return err
		}
	}
	if hasSessions {
		target := filepath.Join(projectDir(defaultProjectID), "sessions")
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if err := os.Rename(legacySessions, target); err != nil {
			return err
		}
	}
	return nil
}

// legacyDomainsHasData reports whether the legacy domains.json file exists and
// contains at least one domain.
func legacyDomainsHasData(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var store domainStore
	if err := json.Unmarshal(data, &store); err != nil {
		return false
	}
	return len(store.Domains) > 0
}

// legacySessionsHasData reports whether the legacy sessions directory exists and
// contains at least one session file.
func legacySessionsHasData(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}
