package doctor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDetectInstallMethod(t *testing.T) {
	c := &Checker{}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: "unknown"},
		{name: "unknown token", path: "unknown", want: "unknown"},
		{name: "homebrew cellar", path: "/opt/homebrew/Cellar/site-health/0.9.5/bin/site-health", want: "Homebrew"},
		{name: "homebrew cellar lowercase", path: "/usr/local/Cellar/site-health/0.9.5/bin/site-health", want: "Homebrew"},
		{name: "homebrew bin", path: "/opt/homebrew/bin/site-health", want: "Homebrew (likely)"},
		{name: "usr local bin", path: "/usr/local/bin/site-health", want: "Homebrew (likely)"},
		{name: "manual", path: "/Users/alice/bin/site-health", want: "manual / pre-built binary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.detectInstallMethod(tt.path)
			if got != tt.want {
				t.Errorf("detectInstallMethod(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethodGoInstall(t *testing.T) {
	c := &Checker{}

	t.Run("GOBIN", func(t *testing.T) {
		t.Setenv("GOBIN", "/Users/alice/go-tools")
		t.Setenv("GOPATH", "")
		got := c.detectInstallMethod("/Users/alice/go-tools/site-health")
		if got != "go install" {
			t.Errorf("got %q, want go install", got)
		}
	})

	t.Run("GOPATH", func(t *testing.T) {
		t.Setenv("GOBIN", "")
		t.Setenv("GOPATH", "/Users/alice/dev")
		got := c.detectInstallMethod("/Users/alice/dev/bin/site-health")
		if got != "go install" {
			t.Errorf("got %q, want go install", got)
		}
	})
}

func TestCheckBinary(t *testing.T) {
	c := &Checker{
		ExecutableFunc: func() (string, error) {
			return "/tmp/site-health", nil
		},
	}

	report := &Report{}
	c.checkBinary(report)

	if report.BinaryPath != "/tmp/site-health" {
		t.Errorf("BinaryPath = %q, want /tmp/site-health", report.BinaryPath)
	}
}

func TestCheckBinaryError(t *testing.T) {
	c := &Checker{
		ExecutableFunc: func() (string, error) {
			return "", fmt.Errorf("not found")
		},
	}

	report := &Report{}
	c.checkBinary(report)

	if report.BinaryPath != "unknown" {
		t.Errorf("BinaryPath = %q, want unknown", report.BinaryPath)
	}
}

func TestFetchLatestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/atillalab/site-health/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"tag_name":"v0.9.5"}`)
	}))
	defer server.Close()

	c := NewChecker()
	c.ReleasesURL = server.URL + "/repos/atillalab/site-health/releases/latest"
	c.HTTPClient = server.Client()

	latest, err := c.fetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("fetchLatestVersion error: %v", err)
	}
	if latest != "0.9.5" {
		t.Errorf("latest = %q, want 0.9.5", latest)
	}
}

func TestFetchLatestVersionHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := NewChecker()
	c.ReleasesURL = server.URL + "/repos/atillalab/site-health/releases/latest"
	c.HTTPClient = server.Client()

	_, err := c.fetchLatestVersion(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestCheckLatestVersionUpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v99.0.0"}`)
	}))
	defer server.Close()

	c := NewChecker()
	c.ReleasesURL = server.URL + "/repos/atillalab/site-health/releases/latest"
	c.HTTPClient = server.Client()

	report := &Report{CurrentVersion: "0.10.0"}
	ctx := context.Background()
	c.checkLatestVersion(ctx, report)

	if !report.UpdateAvailable {
		t.Errorf("expected UpdateAvailable=true")
	}
	if report.LatestVersion != "99.0.0" {
		t.Errorf("LatestVersion = %q, want 99.0.0", report.LatestVersion)
	}
}

func TestCheckSystemTime(t *testing.T) {
	c := &Checker{
		NowFunc: func() time.Time {
			return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
		},
	}

	report := &Report{}
	c.checkSystemTime(report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusOK {
		t.Errorf("expected OK, got %+v", report.Items)
	}
}

func TestCheckSystemTimeBadClock(t *testing.T) {
	c := &Checker{
		NowFunc: func() time.Time {
			return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		},
	}

	report := &Report{}
	c.checkSystemTime(report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusFail {
		t.Errorf("expected FAIL, got %+v", report.Items)
	}
}

func TestCheckDNS(t *testing.T) {
	c := &Checker{
		LookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
			if host == "example.com" {
				return []string{"93.184.216.34"}, nil
			}
			return nil, fmt.Errorf("unexpected host: %s", host)
		},
	}

	report := &Report{}
	c.checkDNS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusOK {
		t.Errorf("expected OK, got %+v", report.Items)
	}
	if !strings.Contains(report.Items[0].Value, "93.184.216.34") {
		t.Errorf("expected IP in value, got %q", report.Items[0].Value)
	}
}

func TestCheckDNSFailure(t *testing.T) {
	c := &Checker{
		LookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
			return nil, fmt.Errorf("lookup failed")
		},
	}

	report := &Report{}
	c.checkDNS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusFail {
		t.Errorf("expected FAIL, got %+v", report.Items)
	}
}

func TestCheckHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "success")
	}))
	defer server.Close()

	c := NewChecker()
	c.ProbeURL = server.URL
	c.HTTPClient = server.Client()

	report := &Report{}
	c.checkHTTPS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusOK {
		t.Errorf("expected OK, got %+v", report.Items)
	}
	if !strings.Contains(report.Items[0].Value, "success") {
		t.Errorf("expected body in value, got %q", report.Items[0].Value)
	}
}

func TestCheckHTTPSFailure(t *testing.T) {
	c := &Checker{
		HTTPClient: &http.Client{
			Timeout: 1 * time.Nanosecond,
		},
	}

	report := &Report{}
	c.checkHTTPS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusFail {
		t.Errorf("expected FAIL, got %+v", report.Items)
	}
}

func TestCheckWHOIS(t *testing.T) {
	c := &Checker{
		DialTimeoutFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			if address == "whois.iana.org:43" {
				return &net.TCPConn{}, nil
			}
			return nil, fmt.Errorf("unexpected address: %s", address)
		},
	}

	report := &Report{}
	c.checkWHOIS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusOK {
		t.Errorf("expected OK, got %+v", report.Items)
	}
}

func TestCheckWHOISFailure(t *testing.T) {
	c := &Checker{
		DialTimeoutFunc: func(network, address string, timeout time.Duration) (net.Conn, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	report := &Report{}
	c.checkWHOIS(context.Background(), report)

	if len(report.Items) != 1 || report.Items[0].Status != StatusFail {
		t.Errorf("expected FAIL, got %+v", report.Items)
	}
}

func TestCheckMailDNS(t *testing.T) {
	c := &Checker{
		LookupMXFunc: func(ctx context.Context, host string) ([]*net.MX, error) {
			return []*net.MX{{Host: "mail.example.com", Pref: 10}}, nil
		},
		LookupTXTFunc: func(ctx context.Context, host string) ([]string, error) {
			switch host {
			case "example.com":
				return []string{"v=spf1 -all"}, nil
			case "_dmarc.example.com":
				return []string{"v=DMARC1; p=reject"}, nil
			default:
				return nil, fmt.Errorf("unexpected host: %s", host)
			}
		},
	}

	report := &Report{}
	c.checkMailDNS(context.Background(), report)

	if len(report.Items) != 3 {
		t.Fatalf("expected 3 mail items, got %d", len(report.Items))
	}
	for _, item := range report.Items {
		if item.Status != StatusOK {
			t.Errorf("%s expected OK, got %s", item.Name, item.Status)
		}
	}
}

func TestCheckMailDNSFailures(t *testing.T) {
	c := &Checker{
		LookupMXFunc: func(ctx context.Context, host string) ([]*net.MX, error) {
			return nil, fmt.Errorf("mx failed")
		},
		LookupTXTFunc: func(ctx context.Context, host string) ([]string, error) {
			return nil, fmt.Errorf("txt failed")
		},
	}

	report := &Report{}
	c.checkMailDNS(context.Background(), report)

	if len(report.Items) != 3 {
		t.Fatalf("expected 3 mail items, got %d", len(report.Items))
	}
	for _, item := range report.Items {
		if item.Status != StatusFail {
			t.Errorf("%s expected FAIL, got %s", item.Name, item.Status)
		}
	}
}

func TestReportOverallStatus(t *testing.T) {
	tests := []struct {
		name   string
		items  []Item
		update bool
		want   string
	}{
		{
			name:   "healthy",
			items:  []Item{{Status: StatusOK}},
			update: false,
			want:   "HEALTHY",
		},
		{
			name:   "warning update",
			items:  []Item{{Status: StatusOK}},
			update: true,
			want:   "WARNING",
		},
		{
			name:   "failure overrides warning",
			items:  []Item{{Status: StatusFail}},
			update: true,
			want:   "UNHEALTHY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Report{Items: tt.items, UpdateAvailable: tt.update}
			if got := r.OverallStatus(); got != tt.want {
				t.Errorf("OverallStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	report := &Report{
		BinaryPath:      "/tmp/site-health",
		InstallMethod:   "manual / pre-built binary",
		CurrentVersion:  "0.9.5",
		LatestVersion:   "0.9.5",
		UpdateAvailable: false,
		Items: []Item{
			{Name: "DNS resolution", Status: StatusOK, Value: "example.com → 93.184.216.34"},
			{Name: "System time", Status: StatusOK, Value: "2026-08-27T12:00:00Z"},
		},
	}

	var buf bytes.Buffer
	Render(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "site-health doctor") {
		t.Errorf("output missing header")
	}
	if !strings.Contains(out, "/tmp/site-health") {
		t.Errorf("output missing binary path")
	}
	if !strings.Contains(out, "HEALTHY") {
		t.Errorf("output missing status")
	}
}

func TestRenderUpdateAvailable(t *testing.T) {
	report := &Report{
		BinaryPath:      "/tmp/site-health",
		InstallMethod:   "manual / pre-built binary",
		CurrentVersion:  "0.10.0",
		LatestVersion:   "0.10.0",
		UpdateAvailable: true,
		Items:           []Item{{Name: "DNS resolution", Status: StatusOK}},
	}

	var buf bytes.Buffer
	Render(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "update available") {
		t.Errorf("output missing update available notice")
	}
	if !strings.Contains(out, "WARNING") {
		t.Errorf("output missing WARNING status")
	}
}

func TestRunUsesDefaults(t *testing.T) {
	c := NewChecker()
	if c.ExecutableFunc == nil {
		t.Error("ExecutableFunc not set")
	}
	if c.NowFunc == nil {
		t.Error("NowFunc not set")
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient not set")
	}
}

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want bool
	}{
		{"0.9.4", "0.10.0", true},
		{"0.10.0", "0.9.4", false},
		{"0.9.4", "0.9.4", false},
		{"0.9.4", "0.9.5", true},
		{"v0.9.4", "0.9.5", true},
		{"1.0.0", "0.10.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := versionLess(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("versionLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
