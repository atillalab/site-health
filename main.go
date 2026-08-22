package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/atillalab/site-health/internal/check"
	"github.com/atillalab/site-health/internal/domain"
	"github.com/atillalab/site-health/internal/output"
	"github.com/atillalab/site-health/internal/version"
)

func main() {
	mailOnly := flag.Bool("mail", false, "Run only mail-related DNS checks")
	verbose := flag.Bool("verbose", false, "Show detailed troubleshooting diagnostics")
	format := flag.String("format", "dashboard", "Output format: dashboard or json")
	expectedURL := flag.String("expected-url", "", "Expected final URL after redirects")
	skipLLMs := flag.Bool("skip-llms-txt", false, "Skip the optional /llms.txt check")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: site-health [--mail] [--verbose] [--expected-url <url>] [--skip-llms-txt] [--format <dashboard|json>] [--version] <domain>\n")
		fmt.Fprintf(os.Stderr, "Example: site-health example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --mail example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --verbose example.com\n")
		fmt.Fprintf(os.Stderr, "Example: site-health --skip-llms-txt example.com\n")
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

	if *format == "text" {
		*format = "dashboard"
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
		Domain:   domainName,
		MailOnly: *mailOnly,
		Verbose:  *verbose,
		SkipLLMs: *skipLLMs,
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
