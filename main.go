package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/atillalab/site-health/internal/check"
	"github.com/atillalab/site-health/internal/config"
	"github.com/atillalab/site-health/internal/doctor"
	"github.com/atillalab/site-health/internal/domain"
	"github.com/atillalab/site-health/internal/output"
	"github.com/atillalab/site-health/internal/version"
)

type boolFlag struct {
	value bool
	set   bool
}

func (f *boolFlag) String() string { return strconv.FormatBool(f.value) }

func (f *boolFlag) Set(value string) error {
	v, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	f.value = v
	f.set = true
	return nil
}

// IsBoolFlag tells the flag package that this flag does not require a value.
// It allows forms like --skip-redirect instead of requiring --skip-redirect=true.
func (f *boolFlag) IsBoolFlag() bool { return true }

type stringFlag struct {
	value string
	set   bool
}

func (f *stringFlag) String() string { return f.value }

func (f *stringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

type mailCheckListFlag struct {
	value string
	set   bool
}

func (f *mailCheckListFlag) String() string {
	return f.value
}

func (f *mailCheckListFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

func main() {
	var (
		verbose        boolFlag
		format         stringFlag
		skipMail       boolFlag
		skipLLMs       boolFlag
		skipRedirect   boolFlag
		whoisOnly      boolFlag
		mailChecks     mailCheckListFlag
		skipMailChecks mailCheckListFlag
	)
	format.value = "dashboard"

	configPath := flag.String("config", "", "Path to config file (default $XDG_CONFIG_HOME/site-health/config.json)")
	mailOnly := flag.Bool("mail", false, "Run only mail-related DNS checks")
	flag.Var(&verbose, "verbose", "Show detailed troubleshooting diagnostics")
	flag.Var(&format, "format", "Output format: dashboard or json")
	expectedHost := flag.String("expected-host", "", "Expected final host after redirects (host or URL); alias for --expected-hosts")
	expectedHosts := flag.String("expected-hosts", "", "Comma-separated expected final hosts after redirects (hosts or URLs)")
	flag.Var(&skipMail, "skip-mail", "Skip mail-related DNS checks in site mode")
	flag.Var(&mailChecks, "mail-checks", "Comma-separated mail checks to run: mx, spf, dmarc")
	flag.Var(&skipMailChecks, "skip-mail-checks", "Comma-separated mail checks to skip: mx, spf, dmarc")
	flag.Var(&skipLLMs, "skip-llms-txt", "Skip the optional /llms.txt check")
	flag.Var(&skipRedirect, "skip-redirect", "Skip the canonical redirect check")
	flag.Var(&whoisOnly, "whois", "Run only WHOIS/domain-registration checks")
	showVersion := flag.Bool("version", false, "Show version and exit")
	initConfig := flag.Bool("init-config", false, "Write a sample config file and exit")
	doctorMode := flag.Bool("doctor", false, "Run self-diagnostics for the binary and environment")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: site-health [--mail] [--whois] [--verbose] [--expected-hosts <hosts>] [--skip-mail] [--mail-checks <mx,spf,dmarc>] [--skip-mail-checks <mx,spf,dmarc>] [--skip-llms-txt] [--skip-redirect] [--format <dashboard|json>] [--config <path>] [--init-config] [--doctor] [--version] [<domain>]\n")
		fmt.Fprintf(os.Stderr, "Example: site-health example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --mail example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --whois example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --mail-checks spf example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-mail-checks spf example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --verbose example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-mail example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-llms-txt example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-redirect example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --expected-hosts example.org example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --expected-hosts example.com,www.example.com example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --format json example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --init-config\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --doctor\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("site-health %s\n", version.Version)
		os.Exit(0)
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}

	if *initConfig {
		if err := writeSampleConfig(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("Created sample config: %s\n", cfgPath)
		os.Exit(0)
	}

	if *doctorMode {
		runDoctor(cfgPath)
		return
	}

	fileCfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid config file %s: %v\n", cfgPath, err)
		os.Exit(2)
	}
	envCfg := config.FromEnv()

	resolvedFormat := config.MergeString(format.value, format.set, "dashboard", envCfg.Format, fileCfg.Format)
	if resolvedFormat == "text" {
		resolvedFormat = "dashboard"
	}
	if resolvedFormat != "dashboard" && resolvedFormat != "json" {
		fmt.Fprintf(os.Stderr, "Error: Unknown format: %s\n", resolvedFormat)
		fmt.Fprintf(os.Stderr, "Supported formats: dashboard, json\n")
		os.Exit(2)
	}

	resolvedVerbose := config.MergeBool(verbose.value, verbose.set, false, envCfg.Verbose, fileCfg.Verbose)
	verboseEnabled := effectiveVerbose(resolvedVerbose, resolvedFormat)

	resolvedSkipMail := config.MergeBool(skipMail.value, skipMail.set, false, envCfg.SkipMail, fileCfg.SkipMail)
	resolvedSkipLLMs := config.MergeBool(skipLLMs.value, skipLLMs.set, false, envCfg.SkipLLMs, fileCfg.SkipLLMs)
	resolvedSkipRedirect := config.MergeBool(skipRedirect.value, skipRedirect.set, false, envCfg.SkipRedirect, fileCfg.SkipRedirect)
	resolvedWhoisOnly := config.MergeBool(whoisOnly.value, whoisOnly.set, false, envCfg.WhoisOnly, fileCfg.WhoisOnly)

	mailCheckNames := splitMailCheckString(mailChecks.value)
	mailCheckNames = config.MergeStringSlice(mailCheckNames, mailChecks.set, envCfg.MailChecks, fileCfg.MailChecks)
	mailChecksSet := mailChecks.set || len(mailCheckNames) > 0

	skipMailCheckNames := splitMailCheckString(skipMailChecks.value)
	skipMailCheckNames = config.MergeStringSlice(skipMailCheckNames, skipMailChecks.set, envCfg.SkipMailChecks, fileCfg.SkipMailChecks)
	skipMailChecksSet := skipMailChecks.set || len(skipMailCheckNames) > 0

	selectedMailChecks, err := resolveMailChecks(mailCheckNames, mailChecksSet, skipMailCheckNames, skipMailChecksSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if err := validateOptions(*mailOnly, resolvedWhoisOnly, resolvedSkipMail, mailChecksSet, skipMailChecksSet, selectedMailChecks); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if verboseEnabled && cfgPath != "" {
		if _, statErr := os.Stat(cfgPath); statErr == nil {
			fmt.Fprintf(os.Stderr, "Using config: %s\n", cfgPath)
		}
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Error: Multiple domains provided.\n")
		flag.Usage()
		os.Exit(2)
	}

	domainName := domain.Normalize(args[0])
	if domainName == "" {
		fmt.Fprintf(os.Stderr, "Error: Invalid domain.\n")
		os.Exit(2)
	}

	runner := &check.Runner{
		Domain:       domainName,
		MailOnly:     *mailOnly,
		WhoisOnly:    resolvedWhoisOnly,
		Verbose:      verboseEnabled,
		SkipMail:     resolvedSkipMail,
		MailChecks:   selectedMailChecks,
		SkipLLMs:     resolvedSkipLLMs,
		SkipRedirect: resolvedSkipRedirect,
	}

	cliExpectedHosts := mergeExpectedHostFlags(*expectedHost, *expectedHosts)
	resolvedExpectedHosts := config.MergeStringSlice(cliExpectedHosts, len(cliExpectedHosts) > 0, envCfg.ExpectedHosts, fileCfg.ExpectedHosts)
	if len(resolvedExpectedHosts) == 0 {
		resolvedExpectedHosts = []string{domainName}
	}
	runner.ExpectedHosts = normalizeExpectedHosts(resolvedExpectedHosts)
	if len(runner.ExpectedHosts) == 0 {
		fmt.Fprintf(os.Stderr, "Error: --expected-hosts must contain at least one valid host or absolute http:// or https:// URL.\n")
		os.Exit(2)
	}

	ctx := context.Background()

	if !*mailOnly && !resolvedWhoisOnly {
		detectForwarding(ctx, runner)
	}

	report := runner.RunChecks(ctx)
	report.Mode = "mail"
	if resolvedWhoisOnly {
		report.Mode = "whois"
	} else if !*mailOnly {
		report.Mode = "site"
	}

	if resolvedFormat == "json" {
		output.RenderJSON(os.Stdout, report)
		if report.Summary.Failures > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if resolvedWhoisOnly {
		output.RenderWhoisDashboard(os.Stdout, report)
	} else if *mailOnly {
		output.RenderMailDashboard(os.Stdout, report)
	} else {
		output.RenderDashboard(os.Stdout, report)
	}

	if report.Summary.Failures > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func validateOptions(mailOnly, whoisOnly, skipMail, mailChecksSet, skipMailChecksSet bool, mailChecks check.MailChecks) error {
	if mailOnly && whoisOnly {
		return fmt.Errorf("--mail and --whois cannot be used together")
	}
	if whoisOnly && (mailChecksSet || skipMailChecksSet) {
		return fmt.Errorf("--whois cannot be used with --mail-checks or --skip-mail-checks")
	}
	if mailOnly && skipMail {
		return fmt.Errorf("--mail and --skip-mail cannot be used together")
	}
	if skipMail && (mailChecksSet || skipMailChecksSet) {
		return fmt.Errorf("--skip-mail cannot be used with --mail-checks or --skip-mail-checks")
	}
	if mailChecksSet && skipMailChecksSet {
		return fmt.Errorf("--mail-checks and --skip-mail-checks cannot be used together")
	}
	if (mailChecksSet || skipMailChecksSet) && mailChecks.EnabledCount() == 0 {
		return fmt.Errorf("no mail checks selected; use --skip-mail to skip all mail checks in site mode")
	}
	return nil
}

func effectiveVerbose(verbose bool, format string) bool {
	return verbose && format != "json"
}

func resolveMailChecks(include []string, includeSet bool, exclude []string, excludeSet bool) (check.MailChecks, error) {
	if includeSet {
		return parseMailCheckSlice(include)
	}

	selected := check.DefaultMailChecks()
	if excludeSet {
		skipped, err := parseMailCheckSlice(exclude)
		if err != nil {
			return check.MailChecks{}, err
		}
		selected.MX = selected.MX && !skipped.MX
		selected.SPF = selected.SPF && !skipped.SPF
		selected.DMARC = selected.DMARC && !skipped.DMARC
	}

	return selected, nil
}

func parseMailCheckList(value string) (check.MailChecks, error) {
	if strings.TrimSpace(value) == "" {
		return check.MailChecks{}, fmt.Errorf("mail check list cannot be empty")
	}

	var names []string
	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		switch name {
		case "":
			return check.MailChecks{}, fmt.Errorf("mail check list cannot contain empty values")
		default:
			names = append(names, name)
		}
	}

	return parseMailCheckSlice(names)
}

func parseMailCheckSlice(names []string) (check.MailChecks, error) {
	if len(names) == 0 {
		return check.MailChecks{}, fmt.Errorf("mail check list cannot be empty")
	}

	var checks check.MailChecks
	for _, name := range names {
		switch name {
		case "mx":
			checks.MX = true
		case "spf":
			checks.SPF = true
		case "dmarc":
			checks.DMARC = true
		default:
			return check.MailChecks{}, fmt.Errorf("unknown mail check %q; supported checks: mx, spf, dmarc", name)
		}
	}

	return checks, nil
}

func splitMailCheckString(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, strings.ToLower(trimmed))
		}
	}
	return out
}

func mergeExpectedHostFlags(expectedHost, expectedHosts string) []string {
	var out []string
	if expectedHost != "" {
		out = append(out, splitAndTrimHosts(expectedHost)...)
	}
	if expectedHosts != "" {
		out = append(out, splitAndTrimHosts(expectedHosts)...)
	}
	return out
}

func splitAndTrimHosts(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeExpectedHosts(hosts []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, h := range hosts {
		host := domain.ParseHost(h)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}

func detectForwarding(ctx context.Context, runner *check.Runner) {
	if len(runner.ExpectedHosts) != 1 || runner.ExpectedHosts[0] != runner.Domain {
		return
	}

	runner.Verbosef("\n\033[1m== Detecting Forwarding ==\033[0m\n")

	var urls []string
	if domain.IsSubdomain(runner.Domain) {
		urls = []string{
			"http://" + runner.Domain,
			"https://" + runner.Domain,
		}
	} else {
		urls = []string{
			"http://" + runner.Domain,
			"http://www." + runner.Domain,
			"https://www." + runner.Domain,
			"https://" + runner.Domain,
		}
	}

	type candidate struct {
		host string
	}

	var mu sync.Mutex
	var candidates []candidate

	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			result := runner.ProbeURLForForwarding(targetURL)
			finalHost := domain.ExtractHost(result.FinalURL)
			if result.StatusCode == 200 && !slices.Contains(runner.ExpectedHosts, finalHost) && !domain.IsSameSiteHost(finalHost, runner.Domain) {
				mu.Lock()
				candidates = append(candidates, candidate{host: finalHost})
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var unique []string
	for _, c := range candidates {
		if !seen[c.host] {
			seen[c.host] = true
			unique = append(unique, c.host)
		}
	}

	if len(unique) == 1 {
		runner.ExpectedHosts = []string{unique[0]}
		runner.ForwardingAutoDetected = true
		runner.Verbosef("Domain forwards to %s\n", unique[0])
		runner.Verbosef("Using forwarded host as expected host for this run\n")
	} else if len(unique) > 1 {
		runner.ForwardingAmbiguous = true
		runner.ForwardingHintHost = unique[0]
		runner.ForwardingCandidates = unique
	}
}

func writeSampleConfig(path string) error {
	if path == "" {
		return fmt.Errorf("could not determine config path")
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access config path: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	sample := `{
  "verbose": false,
  "skip_redirect": false,
  "skip_mail": false,
  "skip_llms_txt": false,
  "whois": false,
  "format": "dashboard",
  "expected_hosts": [],
  "mail_checks": [],
  "skip_mail_checks": []
}
`

	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}
	return nil
}

func runDoctor(configPath string) {
	ctx := context.Background()
	checker := doctor.NewChecker()
	checker.ConfigPath = configPath
	report := checker.Run(ctx)
	doctor.Render(os.Stdout, report)

	if report.HasFailures() {
		os.Exit(1)
	}
	os.Exit(0)
}
