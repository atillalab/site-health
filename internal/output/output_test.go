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

func TestRenderMailDashboardOmitsSkippedChecks(t *testing.T) {
	report := &check.Report{
		Domain: "example.com",
		Mode:   "mail",
		Checks: check.Checks{
			Mail: &check.MailResult{
				Status: check.OK,
				SPF:    &check.SPFResult{Status: check.OK},
			},
		},
		Summary: check.Summary{Status: "HEALTHY"},
	}

	var buf bytes.Buffer
	RenderMailDashboard(&buf, report)

	if bytes.Contains(buf.Bytes(), []byte("MX")) {
		t.Error("output includes skipped MX row")
	}
	if !bytes.Contains(buf.Bytes(), []byte("SPF")) {
		t.Error("output missing SPF row")
	}
	if bytes.Contains(buf.Bytes(), []byte("DMARC")) {
		t.Error("output includes skipped DMARC row")
	}
}

func TestRenderJSONOmitsSkippedMailChecks(t *testing.T) {
	report := &check.Report{
		Domain: "example.com",
		Mode:   "mail",
		Checks: check.Checks{
			Mail: &check.MailResult{
				Status: check.OK,
				SPF:    &check.SPFResult{Status: check.OK},
			},
		},
		Summary: check.Summary{Status: "HEALTHY"},
	}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, report); err != nil {
		t.Fatalf("RenderJSON() unexpected error: %v", err)
	}

	var result struct {
		Checks struct {
			Mail map[string]any `json:"mail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if _, ok := result.Checks.Mail["mx"]; ok {
		t.Error("JSON includes skipped mx check")
	}
	if _, ok := result.Checks.Mail["spf"]; !ok {
		t.Error("JSON missing spf check")
	}
	if _, ok := result.Checks.Mail["dmarc"]; ok {
		t.Error("JSON includes skipped dmarc check")
	}
}
