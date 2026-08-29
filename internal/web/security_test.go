package web

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atillalab/site-health/internal/check"
)

func TestCleanDomainsRejectsNonPublicAndMalformedWebTargets(t *testing.T) {
	inputs := []string{"localhost", "127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.1.1", "::1", "fc00::1", "fe80::1", "example.com\nINJECTED", "example.com:22", "user@example.com", "../../etc/passwd"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			if got := cleanDomains([]string{input}); len(got) != 0 {
				t.Fatalf("web target %q was accepted as %v", input, got)
			}
		})
	}
}

func TestCleanDomainsAcceptsPublicHostname(t *testing.T) {
	if got := cleanDomains([]string{"EXAMPLE.COM"}); len(got) != 1 || got[0] != "EXAMPLE.COM" {
		t.Fatalf("public hostname was not preserved: %v", got)
	}
}

func TestOversizedProjectRequestIsRejected(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()
	payload := `{"name":"` + strings.Repeat("x", 2<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()
	handleProjects(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestProjectIDCannotEscapeDataDirectory(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()
	base, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	path := projectDir("../outside")
	if !strings.HasPrefix(path, filepath.Join(base, "projects")+string(os.PathSeparator)) {
		t.Fatalf("project path escapes data directory: %s", path)
	}
}

func TestStateUsesRestrictivePermissions(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()
	p, err := createProject("Permissions")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dataDirMust(t), projectDir(p.ID), projectDomainsPath(p.ID), projectsIndexPathMust(t)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 && info.Mode().Perm() != 0o600 {
			t.Errorf("%s has permissions %o", path, info.Mode().Perm())
		}
	}
}

func dataDirMust(t *testing.T) string {
	t.Helper()
	p, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func projectsIndexPathMust(t *testing.T) string {
	t.Helper()
	p, err := projectsIndexPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCrossOriginStateChangeIsRejected(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"cross-origin"}`))
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	handleProjects(rec, req)
	if rec.Code == http.StatusCreated || rec.Code == http.StatusOK {
		t.Fatalf("cross-origin state change was accepted: %d", rec.Code)
	}
}

func TestLocalOriginStateChangeIsAccepted(t *testing.T) {
	cleanup := setupTestDataDir(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/projects", strings.NewReader(`{"name":"local"}`))
	req.Host = "localhost"
	req.Header.Set("Origin", "http://localhost")
	rec := httptest.NewRecorder()
	handleProjects(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("local same-origin request rejected: %d", rec.Code)
	}
}

func TestMalformedSessionIDIsRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/projects/valid/sessions/../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handleProjectDetail(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed session ID status = %d", rec.Code)
	}
}

func TestInternalFilesystemErrorsAreNotExposed(t *testing.T) {
	file, err := os.CreateTemp("", "site-health-not-a-directory")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())
	old := dataDirOverride
	dataDirOverride = file.Name()
	defer func() { dataDirOverride = old }()
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handleProjects(rec, req)
	if strings.Contains(rec.Body.String(), file.Name()) {
		t.Fatalf("internal filesystem path exposed in response: %q", rec.Body.String())
	}
}

func TestSecurityHeadersArePresent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handleProjects(rec, req)
	for _, name := range []string{"X-Content-Type-Options", "Referrer-Policy", "Content-Security-Policy"} {
		if rec.Header().Get(name) == "" {
			t.Errorf("missing %s", name)
		}
	}
}

func TestWebResolverRejectsPrivateAnswers(t *testing.T) {
	old := lookupHostIPs
	defer func() { lookupHostIPs = old }()
	lookupHostIPs = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
	}
	if _, err := publicWebDomain("internal.example"); err == nil {
		t.Fatal("private DNS answer accepted")
	}
}

func TestDomainLimit(t *testing.T) {
	for _, n := range []int{maxDomainsPerCheck, maxDomainsPerCheck + 1} {
		domains := make([]string, n)
		for i := range domains {
			domains[i] = "host" + strings.TrimSpace(fmt.Sprint(i)) + ".example.com"
		}
		err := validateDomainCount(domains)
		if n == maxDomainsPerCheck && err != nil {
			t.Fatalf("exact limit rejected: %v", err)
		}
		if n == maxDomainsPerCheck+1 && err == nil {
			t.Fatal("excessive domain count accepted")
		}
	}
}

func TestConcurrentChecksAreBounded(t *testing.T) {
	oldLookup, oldRun := lookupHostIPs, runDomainCheck
	defer func() { lookupHostIPs, runDomainCheck = oldLookup, oldRun }()
	lookupHostIPs = func(_ context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	var active, maxActive int
	var mu sync.Mutex
	runDomainCheck = func(_ context.Context, runner *check.Runner) *check.Report {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return &check.Report{Domain: runner.Domain}
	}
	domains := make([]string, maxConcurrentChecks*2)
	for i := range domains {
		domains[i] = fmt.Sprintf("host%d.example.com", i)
	}
	runChecksForDomains(context.Background(), domains, false)
	if maxActive > maxConcurrentChecks {
		t.Fatalf("observed %d concurrent checks, limit %d", maxActive, maxConcurrentChecks)
	}
}
