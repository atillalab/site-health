package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atillalab/site-health/internal/check"
)

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		mail    bool
		skip    bool
		include bool
		exclude bool
		checks  check.MailChecks
		wantErr bool
	}{
		{name: "site mode", mail: false, skip: false, checks: check.DefaultMailChecks(), wantErr: false},
		{name: "mail mode", mail: true, skip: false, checks: check.DefaultMailChecks(), wantErr: false},
		{name: "skip mail", mail: false, skip: true, checks: check.DefaultMailChecks(), wantErr: false},
		{name: "mail skip conflict", mail: true, skip: true, checks: check.DefaultMailChecks(), wantErr: true},
		{name: "skip mail with include conflict", skip: true, include: true, checks: check.MailChecks{SPF: true}, wantErr: true},
		{name: "skip mail with exclude conflict", skip: true, exclude: true, checks: check.MailChecks{MX: true, DMARC: true}, wantErr: true},
		{name: "include exclude conflict", include: true, exclude: true, checks: check.MailChecks{SPF: true}, wantErr: true},
		{name: "empty selection", include: true, checks: check.MailChecks{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptions(tt.mail, tt.skip, tt.include, tt.exclude, tt.checks)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveVerbose(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		format  string
		want    bool
	}{
		{name: "verbose suppressed for json", verbose: true, format: "json", want: false},
		{name: "verbose enabled for dashboard", verbose: true, format: "dashboard", want: true},
		{name: "verbose enabled for text", verbose: true, format: "text", want: true},
		{name: "verbose disabled for dashboard", verbose: false, format: "dashboard", want: false},
		{name: "verbose disabled for json", verbose: false, format: "json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveVerbose(tt.verbose, tt.format)
			if got != tt.want {
				t.Errorf("effectiveVerbose(%v, %q) = %v, want %v", tt.verbose, tt.format, got, tt.want)
			}
		})
	}
}

func TestParseMailCheckList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    check.MailChecks
		wantErr bool
	}{
		{name: "single", value: "spf", want: check.MailChecks{SPF: true}},
		{name: "multiple", value: "mx,dmarc", want: check.MailChecks{MX: true, DMARC: true}},
		{name: "spaces and case", value: " SPF, dMaRc ", want: check.MailChecks{SPF: true, DMARC: true}},
		{name: "empty", value: "", wantErr: true},
		{name: "empty item", value: "mx,,spf", wantErr: true},
		{name: "unknown", value: "dkim", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMailCheckList(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMailCheckList(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseMailCheckList(%q) = %+v, want %+v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseMailCheckSlice(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    check.MailChecks
		wantErr bool
	}{
		{name: "mx only", values: []string{"mx"}, want: check.MailChecks{MX: true}},
		{name: "all", values: []string{"mx", "spf", "dmarc"}, want: check.DefaultMailChecks()},
		{name: "empty", values: []string{}, wantErr: true},
		{name: "unknown", values: []string{"dkim"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMailCheckSlice(tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMailCheckSlice(%v) error = %v, wantErr %v", tt.values, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseMailCheckSlice(%v) = %+v, want %+v", tt.values, got, tt.want)
			}
		})
	}
}

func TestResolveMailChecks(t *testing.T) {
	tests := []struct {
		name       string
		include    []string
		includeSet bool
		exclude    []string
		excludeSet bool
		want       check.MailChecks
		wantErr    bool
	}{
		{
			name: "default",
			want: check.DefaultMailChecks(),
		},
		{
			name:       "include only spf",
			include:    []string{"spf"},
			includeSet: true,
			want:       check.MailChecks{SPF: true},
		},
		{
			name:       "skip spf",
			exclude:    []string{"spf"},
			excludeSet: true,
			want:       check.MailChecks{MX: true, DMARC: true},
		},
		{
			name:       "empty include errors",
			includeSet: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMailChecks(tt.include, tt.includeSet, tt.exclude, tt.excludeSet)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveMailChecks() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveMailChecks() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBoolFlag(t *testing.T) {
	var f boolFlag
	if f.set {
		t.Error("new boolFlag should not be set")
	}
	if !f.IsBoolFlag() {
		t.Error("boolFlag.IsBoolFlag() should be true")
	}
	if err := f.Set("true"); err != nil {
		t.Fatalf("Set(true) error = %v", err)
	}
	if !f.value || !f.set {
		t.Errorf("after Set(true): value=%v set=%v, want true true", f.value, f.set)
	}
	if err := f.Set("not-bool"); err == nil {
		t.Error("Set(not-bool) expected error")
	}
}

func TestStringFlag(t *testing.T) {
	var f stringFlag
	f.value = "dashboard"
	if f.set {
		t.Error("new stringFlag should not be set")
	}
	if err := f.Set("json"); err != nil {
		t.Fatalf("Set(json) error = %v", err)
	}
	if f.value != "json" || !f.set {
		t.Errorf("after Set(json): value=%q set=%v, want json true", f.value, f.set)
	}
}

func TestMergeExpectedHostFlags(t *testing.T) {
	tests := []struct {
		name          string
		expectedHost  string
		expectedHosts string
		want          []string
	}{
		{name: "empty", want: nil},
		{name: "singular only", expectedHost: "example.com", want: []string{"example.com"}},
		{name: "plural only", expectedHosts: "example.com, www.example.com", want: []string{"example.com", "www.example.com"}},
		{name: "both merged", expectedHost: "example.com", expectedHosts: "www.example.com", want: []string{"example.com", "www.example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeExpectedHostFlags(tt.expectedHost, tt.expectedHosts)
			if len(got) != len(tt.want) {
				t.Errorf("mergeExpectedHostFlags() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mergeExpectedHostFlags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeExpectedHosts(t *testing.T) {
	tests := []struct {
		name  string
		hosts []string
		want  []string
	}{
		{name: "empty", hosts: []string{}, want: nil},
		{name: "single", hosts: []string{"example.com"}, want: []string{"example.com"}},
		{name: "dedupe", hosts: []string{"example.com", "example.com", "www.example.com"}, want: []string{"example.com", "www.example.com"}},
		{name: "extract host from URL", hosts: []string{"https://www.example.com/"}, want: []string{"www.example.com"}},
		{name: "skip invalid", hosts: []string{"example.com", "://bad", "www.example.com"}, want: []string{"example.com", "www.example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeExpectedHosts(tt.hosts)
			if len(got) != len(tt.want) {
				t.Errorf("normalizeExpectedHosts() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeExpectedHosts()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWriteSampleConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site-health", "config.json")

	if err := writeSampleConfig(path); err != nil {
		t.Fatalf("writeSampleConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"format": "dashboard"`) {
		t.Errorf("sample config missing expected content: %s", data)
	}
	if !strings.Contains(string(data), `"expected_hosts": []`) {
		t.Errorf("sample config missing expected_hosts: %s", data)
	}

	if err := writeSampleConfig(path); err == nil {
		t.Error("writeSampleConfig() expected error for existing file")
	}
}

func TestWriteSampleConfigEmptyPath(t *testing.T) {
	if err := writeSampleConfig(""); err == nil {
		t.Error("writeSampleConfig(\"\") expected error")
	}
}
