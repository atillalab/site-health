package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles site-health into a temporary directory and returns the
// path to the binary. It fails the test if the build fails.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "site-health")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// TestInitConfigCLI builds the site-health binary and verifies that the
// --init-config flag creates a sample config file at the path specified by
// --config. This is an integration test; it is skipped when -short is used.
func TestInitConfigCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	bin := buildBinary(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config-dir", "config.json")
	cmd := exec.Command(bin, "--config", configPath, "--init-config")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--init-config failed: %v\n%s", err, out)
	}

	want := "Created sample config: " + configPath
	if !strings.Contains(string(out), want) {
		t.Errorf("output %q does not contain %q", out, want)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !strings.Contains(string(data), `"format": "dashboard"`) {
		t.Errorf("config file missing expected content: %s", data)
	}
	if !strings.Contains(string(data), `"expected_hosts": []`) {
		t.Errorf("config file missing expected_hosts: %s", data)
	}

	// A second run should refuse to overwrite.
	cmd = exec.Command(bin, "--config", configPath, "--init-config")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected second --init-config to fail; got: %s", out)
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("unexpected error output: %s", out)
	}
}

// TestBooleanFlagDoesNotConsumeDomain verifies that boolean flags such as
// --skip-redirect can be used without a value and do not swallow the following
// domain argument.
func TestBooleanFlagDoesNotConsumeDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	bin := buildBinary(t)

	cmd := exec.Command(bin, "--skip-redirect", "example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// example.com may not resolve in all test environments, but the flag
		// parser should not fail with a "strconv.ParseBool" error.
		if strings.Contains(string(out), "strconv.ParseBool") {
			t.Fatalf("boolean flag consumed domain argument: %s", out)
		}
	}
}
