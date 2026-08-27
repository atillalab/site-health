package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/atillalab/site-health/internal/config"
	"github.com/atillalab/site-health/internal/output"
	"github.com/atillalab/site-health/internal/version"
)

// Status represents the result of an individual doctor check.
type Status string

const (
	StatusOK      Status = "OK"
	StatusWarn    Status = "WARN"
	StatusFail    Status = "FAIL"
	StatusUnknown Status = "UNKNOWN"
)

func (s Status) String() string {
	return string(s)
}

// Item is a single environment/installation diagnostic result.
type Item struct {
	Name   string
	Status Status
	Value  string
	Detail string
}

// Report contains the results of a doctor run.
type Report struct {
	BinaryPath      string
	InstallMethod   string
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	UpdateError     string
	Items           []Item
}

// Checker runs self-diagnostics for the site-health binary and environment.
// All fields are optional; nil/empty values default to real stdlib-based
// implementations so production use stays stdlib-only.
type Checker struct {
	ExecutableFunc  func() (string, error)
	NowFunc         func() time.Time
	HTTPClient      *http.Client
	ReleasesURL     string
	ProbeURL        string
	ConfigPath      string
	LookupHostFunc  func(ctx context.Context, host string) ([]string, error)
	LookupTXTFunc   func(ctx context.Context, host string) ([]string, error)
	LookupMXFunc    func(ctx context.Context, host string) ([]*net.MX, error)
	DialTimeoutFunc func(network, address string, timeout time.Duration) (net.Conn, error)
}

const defaultReleasesURL = "https://api.github.com/repos/atillalab/site-health/releases/latest"
const defaultProbeURL = "https://detectportal.firefox.com/success.txt"

// NewChecker returns a Checker configured with real stdlib implementations.
func NewChecker() *Checker {
	return &Checker{
		ExecutableFunc: os.Executable,
		NowFunc:        time.Now,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
				DisableKeepAlives:     true,
			},
		},
		ReleasesURL:     defaultReleasesURL,
		ProbeURL:        defaultProbeURL,
		LookupHostFunc:  net.DefaultResolver.LookupHost,
		LookupTXTFunc:   net.DefaultResolver.LookupTXT,
		LookupMXFunc:    net.DefaultResolver.LookupMX,
		DialTimeoutFunc: net.DialTimeout,
	}
}

// Run executes all doctor checks and returns the report.
func (c *Checker) Run(ctx context.Context) *Report {
	report := &Report{
		CurrentVersion: version.Version,
	}

	c.checkBinary(report)
	c.checkInstallMethod(report)
	c.checkLatestVersion(ctx, report)
	c.checkSystemTime(report)
	c.checkConfigFile(report)
	c.checkEnvVars(report)
	c.checkDNS(ctx, report)
	c.checkHTTPS(ctx, report)
	c.checkWHOIS(ctx, report)
	c.checkMailDNS(ctx, report)

	return report
}

func (c *Checker) executable() (string, error) {
	path, err := c.ExecutableFunc()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, nil
	}
	return resolved, nil
}

func (c *Checker) checkBinary(report *Report) {
	path, err := c.executable()
	if err != nil {
		report.BinaryPath = "unknown"
		return
	}
	report.BinaryPath = path
}

func (c *Checker) checkInstallMethod(report *Report) {
	path := report.BinaryPath
	report.InstallMethod = c.detectInstallMethod(path)
}

func (c *Checker) detectInstallMethod(path string) string {
	if path == "" || path == "unknown" {
		return "unknown"
	}

	lower := strings.ToLower(path)

	// Homebrew installs link from /opt/homebrew/bin/ or /usr/local/bin/ into
	// ../Cellar/site-health/<version>/bin/site-health.
	if strings.Contains(lower, "cellar/site-health") ||
		strings.Contains(lower, "homebrew/cellar/site-health") {
		return "Homebrew"
	}

	// go install typically places binaries in GOPATH/bin or GOBIN.
	goBin := ""
	if v := os.Getenv("GOBIN"); v != "" {
		goBin = filepath.Clean(v)
	}
	goPath := ""
	if v := os.Getenv("GOPATH"); v != "" {
		goPath = filepath.Join(filepath.Clean(v), "bin")
	} else {
		goPath = filepath.Join(c.runtimeOSHomeDir(), "go", "bin")
	}

	binDir := filepath.Dir(path)
	if (goBin != "" && binDir == goBin) || (goPath != "" && binDir == goPath) {
		return "go install"
	}

	// Homebrew symlink in a standard Homebrew bin directory.
	if strings.HasPrefix(lower, "/opt/homebrew/bin/") || strings.HasPrefix(lower, "/usr/local/bin/") {
		return "Homebrew (likely)"
	}

	return "manual / pre-built binary"
}

// runtimeOSHomeDir is a tiny shim for testing; on real systems it matches
// os.UserHomeDir behavior used by the Go toolchain for the default GOPATH.
func (c *Checker) runtimeOSHomeDir() string {
	dir, _ := os.UserHomeDir()
	return dir
}

func (c *Checker) checkLatestVersion(ctx context.Context, report *Report) {
	latest, err := c.fetchLatestVersion(ctx)
	if err != nil {
		report.LatestVersion = "unknown"
		report.UpdateError = err.Error()
		return
	}

	report.LatestVersion = latest
	report.UpdateAvailable = latest != "" && versionLess(report.CurrentVersion, latest)
}

// versionLess reports whether a is a lower semver version than b. It expects
// dot-separated numeric version strings (e.g. "0.10.0") and falls back to
// simple string inequality for non-numeric or malformed inputs.
func versionLess(a, b string) bool {
	partsA := strings.Split(strings.TrimPrefix(a, "v"), ".")
	partsB := strings.Split(strings.TrimPrefix(b, "v"), ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA < numB
		}
	}

	return false
}

func (c *Checker) fetchLatestVersion(ctx context.Context) (string, error) {
	url := c.ReleasesURL
	if url == "" {
		url = defaultReleasesURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "site-health/"+version.Version)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	return strings.TrimPrefix(payload.TagName, "v"), nil
}

func (c *Checker) checkSystemTime(report *Report) {
	now := c.NowFunc()
	value := now.UTC().Format(time.RFC3339)
	status := StatusOK
	detail := ""

	// A system clock before 2020 strongly suggests the clock is wrong, which
	// would cause TLS certificate validation failures.
	if now.Year() < 2020 {
		status = StatusFail
		detail = "system clock appears to be set incorrectly"
	}

	report.Items = append(report.Items, Item{
		Name:   "System time",
		Status: status,
		Value:  value,
		Detail: detail,
	})
}

func (c *Checker) checkConfigFile(report *Report) {
	path := c.ConfigPath
	if path == "" {
		path = config.DefaultPath()
	}
	if path == "" {
		report.Items = append(report.Items, Item{
			Name:   "Config file",
			Status: StatusUnknown,
			Value:  "unknown",
			Detail: "could not determine config path",
		})
		return
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			report.Items = append(report.Items, Item{
				Name:   "Config file",
				Status: StatusOK,
				Value:  "not configured",
				Detail: path,
			})
			return
		}
		report.Items = append(report.Items, Item{
			Name:   "Config file",
			Status: StatusFail,
			Value:  "unreadable",
			Detail: err.Error(),
		})
		return
	}

	if _, err := config.Load(path); err != nil {
		report.Items = append(report.Items, Item{
			Name:   "Config file",
			Status: StatusFail,
			Value:  "invalid",
			Detail: err.Error(),
		})
		return
	}

	report.Items = append(report.Items, Item{
		Name:   "Config file",
		Status: StatusOK,
		Value:  path,
	})
}

func (c *Checker) checkEnvVars(report *Report) {
	var set []string
	var warnings []string

	for _, ev := range config.SupportedEnvVars() {
		v, ok := os.LookupEnv(ev.Name)
		if !ok {
			continue
		}
		set = append(set, fmt.Sprintf("%s=%s", ev.Name, v))
		if ev.Kind == "bool" {
			if _, err := strconv.ParseBool(v); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s=%q is not a valid boolean", ev.Name, v))
			}
		}
	}

	status := StatusOK
	value := "none set"
	detail := ""

	if len(set) > 0 {
		value = fmt.Sprintf("%d set", len(set))
		detail = strings.Join(set, ", ")
	}
	if len(warnings) > 0 {
		status = StatusWarn
		if detail != "" {
			detail += "; "
		}
		detail += strings.Join(warnings, "; ")
	}

	report.Items = append(report.Items, Item{
		Name:   "Env vars",
		Status: status,
		Value:  value,
		Detail: detail,
	})
}

func (c *Checker) checkDNS(ctx context.Context, report *Report) {
	addrs, err := c.LookupHostFunc(ctx, "example.com")
	if err != nil {
		report.Items = append(report.Items, Item{
			Name:   "DNS resolution",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		})
		return
	}

	report.Items = append(report.Items, Item{
		Name:   "DNS resolution",
		Status: StatusOK,
		Value:  fmt.Sprintf("example.com → %s", strings.Join(addrs, ", ")),
	})
}

func (c *Checker) checkHTTPS(ctx context.Context, report *Report) {
	url := c.ProbeURL
	if url == "" {
		url = defaultProbeURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		report.Items = append(report.Items, Item{
			Name:   "Outbound HTTPS",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		})
		return
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		status := StatusFail
		detail := err.Error()
		if strings.Contains(detail, "certificate") || strings.Contains(detail, "x509") {
			// TLS root CA / certificate problem.
			status = StatusFail
		}
		report.Items = append(report.Items, Item{
			Name:   "Outbound HTTPS",
			Status: status,
			Value:  "failed",
			Detail: detail,
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		report.Items = append(report.Items, Item{
			Name:   "Outbound HTTPS",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		})
		return
	}

	status := StatusOK
	detail := ""
	bodyStr := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK {
		status = StatusFail
		detail = fmt.Sprintf("expected status 200, got %d", resp.StatusCode)
	} else if bodyStr == "" {
		status = StatusWarn
		detail = "response body was empty"
	}

	value := fmt.Sprintf("%s → %d", url, resp.StatusCode)
	if bodyStr != "" && len(bodyStr) <= 32 {
		value = fmt.Sprintf("%s → %d (%s)", url, resp.StatusCode, bodyStr)
	}

	report.Items = append(report.Items, Item{
		Name:   "Outbound HTTPS",
		Status: status,
		Value:  value,
		Detail: detail,
	})
}

func (c *Checker) checkWHOIS(ctx context.Context, report *Report) {
	conn, err := c.DialTimeoutFunc("tcp", "whois.iana.org:43", 5*time.Second)
	if err != nil {
		report.Items = append(report.Items, Item{
			Name:   "WHOIS lookup",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		})
		return
	}
	_ = conn.Close()

	report.Items = append(report.Items, Item{
		Name:   "WHOIS lookup",
		Status: StatusOK,
		Value:  "whois.iana.org:43 reachable",
	})
}

func (c *Checker) checkMailDNS(ctx context.Context, report *Report) {
	report.Items = append(report.Items, c.checkMailMX(ctx))
	report.Items = append(report.Items, c.checkMailSPF(ctx))
	report.Items = append(report.Items, c.checkMailDMARC(ctx))
}

func (c *Checker) checkMailMX(ctx context.Context) Item {
	records, err := c.LookupMXFunc(ctx, "example.com")
	if err != nil {
		return Item{
			Name:   "Mail DNS (MX)",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		}
	}
	if len(records) == 0 {
		return Item{
			Name:   "Mail DNS (MX)",
			Status: StatusFail,
			Value:  "no records",
			Detail: "MX lookup returned no records",
		}
	}
	return Item{
		Name:   "Mail DNS (MX)",
		Status: StatusOK,
		Value:  fmt.Sprintf("%d record(s)", len(records)),
	}
}

func (c *Checker) checkMailSPF(ctx context.Context) Item {
	records, err := c.LookupTXTFunc(ctx, "example.com")
	if err != nil {
		return Item{
			Name:   "Mail DNS (SPF)",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		}
	}
	for _, rec := range records {
		if strings.HasPrefix(strings.ToLower(rec), "v=spf1") {
			return Item{
				Name:   "Mail DNS (SPF)",
				Status: StatusOK,
				Value:  "record found",
			}
		}
	}
	return Item{
		Name:   "Mail DNS (SPF)",
		Status: StatusOK,
		Value:  "no SPF record",
		Detail: "example.com has no SPF policy (not required for doctor)",
	}
}

func (c *Checker) checkMailDMARC(ctx context.Context) Item {
	records, err := c.LookupTXTFunc(ctx, "_dmarc.example.com")
	if err != nil {
		return Item{
			Name:   "Mail DNS (DMARC)",
			Status: StatusFail,
			Value:  "failed",
			Detail: err.Error(),
		}
	}
	for _, rec := range records {
		if strings.HasPrefix(strings.ToLower(rec), "v=dmarc1") {
			return Item{
				Name:   "Mail DNS (DMARC)",
				Status: StatusOK,
				Value:  "record found",
			}
		}
	}
	return Item{
		Name:   "Mail DNS (DMARC)",
		Status: StatusOK,
		Value:  "no DMARC record",
		Detail: "_dmarc.example.com has no DMARC policy (not required for doctor)",
	}
}

// HasFailures returns true if any item failed.
func (r *Report) HasFailures() bool {
	for _, item := range r.Items {
		if item.Status == StatusFail {
			return true
		}
	}
	return false
}

// OverallStatus returns the overall doctor status string.
func (r *Report) OverallStatus() string {
	if r.HasFailures() {
		return "UNHEALTHY"
	}
	if r.UpdateAvailable {
		return "WARNING"
	}
	return "HEALTHY"
}

// Render writes a dashboard-style report to w.
func Render(w io.Writer, report *Report) {
	fmt.Fprintf(w, "%ssite-health doctor%s\n", output.Bold, output.Reset)
	fmt.Fprintln(w, "──────────────────")

	fmt.Fprintf(w, "Binary path:      %s\n", report.BinaryPath)
	fmt.Fprintf(w, "Install method:   %s\n", report.InstallMethod)
	fmt.Fprintf(w, "Current version:  %s\n", report.CurrentVersion)

	latestValue := report.LatestVersion
	if latestValue == "" {
		latestValue = "unknown"
	}
	if report.UpdateAvailable {
		fmt.Fprintf(w, "Latest version:   %s    %s⚠ update available%s\n", latestValue, output.Yellow, output.Reset)
	} else if report.UpdateError != "" {
		fmt.Fprintf(w, "Latest version:   %s    (%s)\n", latestValue, report.UpdateError)
	} else {
		fmt.Fprintf(w, "Latest version:   %s    (up to date)\n", latestValue)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%sEnvironment%s\n", output.Bold, output.Reset)
	fmt.Fprintln(w, "───────────")

	for _, item := range report.Items {
		renderItem(w, item)
	}

	fmt.Fprintf(w, "Status: %s%s%s\n", output.StatusColor(report.OverallStatus()), report.OverallStatus(), output.Reset)
}

func renderItem(w io.Writer, item Item) {
	value := item.Value
	if value == "" {
		value = item.Status.String()
	}

	switch item.Status {
	case StatusOK:
		fmt.Fprintf(w, "  %-18s %s  %s\n", item.Name, output.StatusColor("OK")+"OK"+output.Reset, value)
	case StatusWarn:
		fmt.Fprintf(w, "  %-18s %s⚠ WARN%s  %s\n", item.Name, output.Yellow, output.Reset, value)
	case StatusFail:
		fmt.Fprintf(w, "  %-18s %s✖ FAIL%s  %s\n", item.Name, output.Red, output.Reset, value)
	default:
		fmt.Fprintf(w, "  %-18s %s  %s\n", item.Name, item.Status, value)
	}

	if item.Detail != "" {
		fmt.Fprintf(w, "                     %s\n", item.Detail)
	}
}
