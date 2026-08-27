package check

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func TestGetTLDDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "com"},
		{"example.co.uk", "uk"},
		{"test", ""},
		{"example.io", "io"},
		{"sub.example.org", "org"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getTLDDomain(tt.input)
			if result != tt.expected {
				t.Errorf("getTLDDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIANAWhoisReferral(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "refer",
			input:    "refer: whois.example",
			expected: "whois.example",
		},
		{
			name:     "whois",
			input:    "whois: whois.nic.sh",
			expected: "whois.nic.sh",
		},
		{
			name:     "whois server",
			input:    "Whois Server: whois.example",
			expected: "whois.example",
		},
		{
			name:     "mixed case and whitespace",
			input:    "  WhOiS SeRvEr  :  whois.example.net  ",
			expected: "whois.example.net",
		},
		{
			name:     "unrelated field",
			input:    "remarks: Registration information: https://example.test/",
			expected: "",
		},
		{
			name:     "not colon delimited",
			input:    "whois.example",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ianaWhoisReferral(tt.input)
			if result != tt.expected {
				t.Errorf("ianaWhoisReferral(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseExpiryDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2025-01-01T00:00:00Z", false},
		{"2025-01-01 12:00:00", false},
		{"2025-01-01", false},
		{"01-Jan-2025", false},
		{"2025/01/01", false},
		{"invalid-date", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseExpiryDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseExpiryDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseExpiryDateValues(t *testing.T) {
	input := "2025-06-15"
	expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	result, err := parseExpiryDate(input)
	if err != nil {
		t.Fatalf("parseExpiryDate(%q) unexpected error: %v", input, err)
	}

	if !result.Equal(expected) {
		t.Errorf("parseExpiryDate(%q) = %v, want %v", input, result, expected)
	}
}

func TestDomainRegistrationStatus(t *testing.T) {
	tests := []struct {
		name          string
		daysRemaining int
		expected      Status
	}{
		{name: "expired", daysRemaining: -1, expected: FAIL},
		{name: "fail boundary", daysRemaining: 14, expected: FAIL},
		{name: "warn lower boundary", daysRemaining: 15, expected: WARN},
		{name: "warn boundary", daysRemaining: 60, expected: WARN},
		{name: "ok", daysRemaining: 61, expected: OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainRegistrationStatus(tt.daysRemaining)
			if result != tt.expected {
				t.Errorf("domainRegistrationStatus(%d) = %s, want %s", tt.daysRemaining, result, tt.expected)
			}
		})
	}
}

func TestLogDomainRegistrationDetailsIncludesExpiryDate(t *testing.T) {
	oldStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() unexpected error: %v", err)
	}
	defer func() {
		os.Stderr = oldStderr
		readPipe.Close()
	}()

	os.Stderr = writePipe

	runner := &Runner{Verbose: true}
	runner.logDomainRegistrationDetails(&DomainRegistrationResult{
		Registrar: "Example Registrar",
		ExpiresAt: "2028-09-14T04:00:00Z",
	})

	writePipe.Close()

	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("io.ReadAll() unexpected error: %v", err)
	}

	if !bytes.Contains(output, []byte("registrar: Example Registrar")) {
		t.Error("verbose output missing registrar")
	}
	if !bytes.Contains(output, []byte("expiry date: 2028-09-14T04:00:00Z")) {
		t.Error("verbose output missing expiry date")
	}
}
