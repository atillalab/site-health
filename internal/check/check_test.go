package check

import (
	"testing"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{OK, "OK"},
		{WARN, "WARN"},
		{FAIL, "FAIL"},
		{Status(99), "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Status(%d).String() = %q, want %q", int(tt.status), result, tt.expected)
			}
		})
	}
}

func TestStatusMarshalJSON(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{OK, `"OK"`},
		{WARN, `"WARN"`},
		{FAIL, `"FAIL"`},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result, err := tt.status.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() unexpected error: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("MarshalJSON() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestEscalate(t *testing.T) {
	tests := []struct {
		current  Status
		incoming Status
		expected Status
	}{
		{OK, OK, OK},
		{OK, WARN, WARN},
		{OK, FAIL, FAIL},
		{WARN, OK, WARN},
		{WARN, WARN, WARN},
		{WARN, FAIL, FAIL},
		{FAIL, OK, FAIL},
		{FAIL, WARN, FAIL},
		{FAIL, FAIL, FAIL},
	}

	for _, tt := range tests {
		name := tt.current.String() + "+" + tt.incoming.String()
		t.Run(name, func(t *testing.T) {
			result := Escalate(tt.current, tt.incoming)
			if result != tt.expected {
				t.Errorf("Escalate(%v, %v) = %v, want %v", tt.current, tt.incoming, result, tt.expected)
			}
		})
	}
}

func TestRunnerFailAndWarn(t *testing.T) {
	r := &Runner{Domain: "example.com"}

	if r.failCount != 0 || r.warnCount != 0 {
		t.Fatalf("initial counts should be 0, got fail=%d warn=%d", r.failCount, r.warnCount)
	}

	r.Fail("test failure")
	r.Fail("another failure")
	r.Warn("test warning")

	if r.failCount != 2 {
		t.Errorf("failCount = %d, want 2", r.failCount)
	}
	if r.warnCount != 1 {
		t.Errorf("warnCount = %d, want 1", r.warnCount)
	}
	if len(r.issues) != 3 {
		t.Errorf("issues count = %d, want 3", len(r.issues))
	}
}

func TestRunnerOverallStatus(t *testing.T) {
	tests := []struct {
		name     string
		fails    int
		warns    int
		expected string
	}{
		{"healthy", 0, 0, "HEALTHY"},
		{"warning", 0, 1, "WARNING"},
		{"unhealthy with fails", 1, 0, "UNHEALTHY"},
		{"unhealthy with both", 1, 1, "UNHEALTHY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{Domain: "example.com"}
			for i := 0; i < tt.fails; i++ {
				r.Fail("fail")
			}
			for i := 0; i < tt.warns; i++ {
				r.Warn("warn")
			}
			result := r.OverallStatus()
			if result != tt.expected {
				t.Errorf("OverallStatus() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRunnerBuildReport(t *testing.T) {
	r := &Runner{
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
	}
	r.Fail("test issue")

	report := r.buildReport("site")

	if report.Tool != "site-health" {
		t.Errorf("Tool = %q, want 'site-health'", report.Tool)
	}
	if report.Domain != "example.com" {
		t.Errorf("Domain = %q, want 'example.com'", report.Domain)
	}
	if report.Mode != "site" {
		t.Errorf("Mode = %q, want 'site'", report.Mode)
	}
	if report.Summary.Failures != 1 {
		t.Errorf("Summary.Failures = %d, want 1", report.Summary.Failures)
	}
	if report.Summary.Status != "UNHEALTHY" {
		t.Errorf("Summary.Status = %q, want 'UNHEALTHY'", report.Summary.Status)
	}
}
