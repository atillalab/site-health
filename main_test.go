package main

import (
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

func TestResolveMailChecks(t *testing.T) {
	tests := []struct {
		name string
		run  mailCheckListFlag
		skip mailCheckListFlag
		want check.MailChecks
	}{
		{
			name: "default",
			want: check.DefaultMailChecks(),
		},
		{
			name: "include only spf",
			run:  mailCheckListFlag{value: "spf", set: true},
			want: check.MailChecks{SPF: true},
		},
		{
			name: "skip spf",
			skip: mailCheckListFlag{value: "spf", set: true},
			want: check.MailChecks{MX: true, DMARC: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMailChecks(tt.run, tt.skip)
			if err != nil {
				t.Fatalf("resolveMailChecks() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveMailChecks() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
