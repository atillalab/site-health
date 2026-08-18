package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/atillalab/site-health/internal/check"
)

func TestRenderJSON(t *testing.T) {
	report := &check.Report{
		Tool:    "site-health",
		Version: "0.8",
		Domain:  "example.com",
		Mode:    "mail",
		Summary: check.Summary{
			Status:   "HEALTHY",
			Failures: 0,
			Warnings: 0,
		},
	}

	var buf bytes.Buffer
	err := RenderJSON(&buf, report)
	if err != nil {
		t.Fatalf("RenderJSON() unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["tool"] != "site-health" {
		t.Errorf("tool = %v, want 'site-health'", result["tool"])
	}
	if result["domain"] != "example.com" {
		t.Errorf("domain = %v, want 'example.com'", result["domain"])
	}
}

func TestRenderDashboard(t *testing.T) {
	report := &check.Report{
		Tool:    "site-health",
		Version: "0.8",
		Domain:  "example.com",
		Mode:    "site",
		Summary: check.Summary{
			Status:   "HEALTHY",
			Failures: 0,
			Warnings: 0,
		},
	}

	var buf bytes.Buffer
	RenderDashboard(&buf, report)

	if !bytes.Contains(buf.Bytes(), []byte("Site Health Check")) {
		t.Error("output missing 'Site Health Check'")
	}
	if !bytes.Contains(buf.Bytes(), []byte("example.com")) {
		t.Error("output missing domain")
	}
}
