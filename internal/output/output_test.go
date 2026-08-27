package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/atillalab/site-health/internal/check"
)

func TestRenderJSON(t *testing.T) {
	domainRegDays := 757
	report := &check.Report{
		Tool:    "site-health",
		Version: "0.8",
		Domain:  "example.com",
		Mode:    "mail",
		Checks: check.Checks{
			DomainRegistration: &check.DomainRegistrationResult{
				Status:        check.OK,
				ExpiresAt:     "2028-09-14T04:00:00Z",
				DaysRemaining: &domainRegDays,
			},
		},
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

	checks, ok := result["checks"].(map[string]any)
	if !ok {
		t.Fatalf("checks missing or invalid: %v", result["checks"])
	}

	domainRegistration, ok := checks["domain_registration"].(map[string]any)
	if !ok {
		t.Fatalf("domain_registration missing or invalid: %v", checks["domain_registration"])
	}

	if domainRegistration["expires_at"] != "2028-09-14T04:00:00Z" {
		t.Errorf("expires_at = %v, want '2028-09-14T04:00:00Z'", domainRegistration["expires_at"])
	}
	if domainRegistration["days_remaining"] != float64(757) {
		t.Errorf("days_remaining = %v, want 757", domainRegistration["days_remaining"])
	}
}

func TestRenderDashboard(t *testing.T) {
	domainRegDays := 224
	report := &check.Report{
		Tool:    "site-health",
		Version: "0.8",
		Domain:  "example.com",
		Mode:    "site",
		Checks: check.Checks{
			DomainRegistration: &check.DomainRegistrationResult{
				Status:        check.OK,
				ExpiresAt:     "2027-05-07T00:00:00Z",
				DaysRemaining: &domainRegDays,
			},
		},
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
	if !bytes.Contains(buf.Bytes(), []byte("Domain Reg   224 days (07 May 2027)")) {
		t.Error("output missing domain registration expiry date and days")
	}
}

func TestRenderDashboardDomainRegistrationExpiryExamples(t *testing.T) {
	tests := []struct {
		name     string
		result   check.DomainRegistrationResult
		expected string
		status   string
	}{
		{
			name: "ok",
			result: check.DomainRegistrationResult{
				Status:        check.OK,
				ExpiresAt:     "2027-05-07T00:00:00Z",
				DaysRemaining: intPtr(224),
			},
			expected: "Domain Reg   224 days (07 May 2027)",
			status:   "",
		},
		{
			name: "warn",
			result: check.DomainRegistrationResult{
				Status:        check.WARN,
				ExpiresAt:     "2026-10-15T00:00:00Z",
				DaysRemaining: intPtr(42),
			},
			expected: "Domain Reg   42 days (15 Oct 2026)",
			status:   "WARN",
		},
		{
			name: "fail",
			result: check.DomainRegistrationResult{
				Status:        check.FAIL,
				ExpiresAt:     "2026-09-15T00:00:00Z",
				DaysRemaining: intPtr(12),
			},
			expected: "Domain Reg   12 days (15 Sep 2026)",
			status:   "FAIL",
		},
		{
			name: "unknown",
			result: check.DomainRegistrationResult{
				Status: check.WARN,
			},
			expected: "Domain Reg   Unknown",
			status:   "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &check.Report{
				Domain: "example.com",
				Mode:   "site",
				Checks: check.Checks{
					DomainRegistration: &tt.result,
				},
				Summary: check.Summary{Status: "WARNING"},
			}

			var buf bytes.Buffer
			RenderDashboard(&buf, report)

			if !bytes.Contains(buf.Bytes(), []byte(tt.expected)) {
				t.Errorf("output missing %q\n%s", tt.expected, buf.String())
			}
			if tt.status != "" && !bytes.Contains(buf.Bytes(), []byte(tt.status)) {
				t.Errorf("output missing status %q\n%s", tt.status, buf.String())
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
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
