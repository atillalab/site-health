package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/atillalab/site-health/internal/check"
	"github.com/atillalab/site-health/internal/domain"
	"github.com/atillalab/site-health/internal/output"
	"github.com/atillalab/site-health/internal/version"
)

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
	var mailChecks mailCheckListFlag
	var skipMailChecks mailCheckListFlag

	mailOnly := flag.Bool("mail", false, "Run only mail-related DNS checks")
	verbose := flag.Bool("verbose", false, "Show detailed troubleshooting diagnostics")
	format := flag.String("format", "dashboard", "Output format: dashboard or json")
	expectedURL := flag.String("expected-url", "", "Expected final URL after redirects")
	skipMail := flag.Bool("skip-mail", false, "Skip mail-related DNS checks in site mode")
	flag.Var(&mailChecks, "mail-checks", "Comma-separated mail checks to run: mx, spf, dmarc")
	flag.Var(&skipMailChecks, "skip-mail-checks", "Comma-separated mail checks to skip: mx, spf, dmarc")
	skipLLMs := flag.Bool("skip-llms-txt", false, "Skip the optional /llms.txt check")
	skipRedirect := flag.Bool("skip-redirect", false, "Skip the canonical redirect check")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: site-health [--mail] [--verbose] [--expected-url <url>] [--skip-mail] [--mail-checks <mx,spf,dmarc>] [--skip-mail-checks <mx,spf,dmarc>] [--skip-llms-txt] [--skip-redirect] [--format <dashboard|json>] [--version] <domain>\n")
		fmt.Fprintf(os.Stderr, "Example: site-health example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --mail example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --mail-checks spf example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-mail-checks spf example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --verbose example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-mail example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-llms-txt example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-redirect example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --expected-url https://example.org/ example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --format json example.com\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("site-health %s\n", version.Version)
		os.Exit(0)
	}

	if *format != "dashboard" && *format != "json" && *format != "text" {
		fmt.Fprintf(os.Stderr, "Error: Unknown format: %s\n", *format)
		fmt.Fprintf(os.Stderr, "Supported formats: dashboard, json\n")
		os.Exit(2)
	}

	selectedMailChecks, err := resolveMailChecks(mailChecks, skipMailChecks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if err := validateOptions(*mailOnly, *skipMail, mailChecks.set, skipMailChecks.set, selectedMailChecks); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if *format == "text" {
		*format = "dashboard"
	}
	verboseEnabled := effectiveVerbose(*verbose, *format)

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
		Verbose:      verboseEnabled,
		SkipMail:     *skipMail,
		MailChecks:   selectedMailChecks,
		SkipLLMs:     *skipLLMs,
		SkipRedirect: *skipRedirect,
	}

	if *expectedURL != "" {
		normalized, err := domain.NormalizeURL(*expectedURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: --expected-url must be a valid absolute http:// or https:// URL.\n")
			os.Exit(2)
		}
		runner.ExpectedURL = normalized
	} else {
		runner.ExpectedURL = "https://" + domainName + "/"
	}

	ctx := context.Background()

	if !*mailOnly {
		detectForwarding(ctx, runner)
	}

	report := runner.RunChecks(ctx)
	report.Mode = "mail"
	if !*mailOnly {
		report.Mode = "site"
	}

	if *format == "json" {
		output.RenderJSON(os.Stdout, report)
		if report.Summary.Failures > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *mailOnly {
		output.RenderMailDashboard(os.Stdout, report)
	} else {
		output.RenderDashboard(os.Stdout, report)
	}

	if report.Summary.Failures > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func validateOptions(mailOnly, skipMail, mailChecksSet, skipMailChecksSet bool, mailChecks check.MailChecks) error {
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

func resolveMailChecks(mailChecks, skipMailChecks mailCheckListFlag) (check.MailChecks, error) {
	if mailChecks.set {
		return parseMailCheckList(mailChecks.value)
	}

	selected := check.DefaultMailChecks()
	if skipMailChecks.set {
		skipped, err := parseMailCheckList(skipMailChecks.value)
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
	var checks check.MailChecks
	if strings.TrimSpace(value) == "" {
		return checks, fmt.Errorf("mail check list cannot be empty")
	}

	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		switch name {
		case "":
			return check.MailChecks{}, fmt.Errorf("mail check list cannot contain empty values")
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

func detectForwarding(ctx context.Context, runner *check.Runner) {
	if runner.ExpectedURL != "https://"+runner.Domain+"/" {
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
		url string
	}

	var mu sync.Mutex
	var candidates []candidate

	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			result := runner.ProbeURLForForwarding(targetURL)
			if result.StatusCode == 200 && result.FinalURL != runner.ExpectedURL && !domain.IsSameSiteHost(domain.ExtractHost(result.FinalURL), runner.Domain) {
				mu.Lock()
				candidates = append(candidates, candidate{url: result.FinalURL})
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	seen := make(map[string]bool)
	var unique []string
	for _, c := range candidates {
		if !seen[c.url] {
			seen[c.url] = true
			unique = append(unique, c.url)
		}
	}

	if len(unique) == 1 {
		runner.ExpectedURL = unique[0]
		runner.ForwardingAutoDetected = true
		runner.Verbosef("Domain forwards to %s\n", unique[0])
		runner.Verbosef("Using forwarded URL as expected URL for this run\n")
	} else if len(unique) > 1 {
		runner.ForwardingAmbiguous = true
		runner.ForwardingHintURL = unique[0]
		runner.ForwardingCandidates = unique
	}
}
