package check

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/atillalab/site-health/internal/domain"
)

type probeResult struct {
	StatusCode int
	FinalURL   string
	Redirects  int
	TotalTime  time.Duration
	Error      error
}

const maxContentBody = 5 << 20

func readLimitedBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxContentBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxContentBody {
		return nil, fmt.Errorf("response body exceeds %d byte limit", maxContentBody)
	}
	return data, nil
}

func (r *Runner) httpClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DisableKeepAlives: true}
	if r.PublicOnly {
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, addr := range addrs {
				if addr.IP.IsLoopback() || addr.IP.IsPrivate() || addr.IP.IsLinkLocalUnicast() || addr.IP.IsUnspecified() {
					return nil, fmt.Errorf("blocked private or local address")
				}
			}
			if len(addrs) == 0 {
				return nil, fmt.Errorf("host did not resolve")
			}
			return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
		}
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if r.PublicOnly {
			if ip := net.ParseIP(req.URL.Hostname()); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
				return fmt.Errorf("redirect to private or local address blocked")
			}
			if req.URL.Hostname() == "localhost" || strings.HasSuffix(req.URL.Hostname(), ".localhost") {
				return fmt.Errorf("redirect to private or local address blocked")
			}
		}
		return nil
	}}
}

func (r *Runner) probeURL(targetURL string) probeResult {
	redirectCount := 0

	client := r.httpClient()
	oldRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		redirectCount = len(via)
		return oldRedirect(req, via)
	}

	start := time.Now()
	resp, err := client.Get(targetURL)
	elapsed := time.Since(start)

	if err != nil {
		return probeResult{Error: err, TotalTime: elapsed}
	}
	defer resp.Body.Close()

	return probeResult{
		StatusCode: resp.StatusCode,
		FinalURL:   resp.Request.URL.String(),
		Redirects:  redirectCount,
		TotalTime:  elapsed,
	}
}

func (r *Runner) CheckHTTP() (redirectStatus, httpsStatus, responseStatus Status, responseMs *int) {
	return r.checkHTTPWithProbe(r.probeURL)
}

func (r *Runner) checkHTTPWithProbe(probe func(string) probeResult) (redirectStatus, httpsStatus, responseStatus Status, responseMs *int) {
	if r.SkipRedirect {
		redirectStatus = SKIP
	} else {
		redirectStatus = OK
	}
	httpsStatus = OK
	responseStatus = OK

	r.Verbosef("\n\033[1m== HTTP and Redirects ==\033[0m\n")

	var urls []string
	if domain.IsSubdomain(r.Domain) {
		urls = []string{
			"http://" + r.Domain,
			"https://" + r.Domain,
		}
	} else {
		urls = []string{
			"http://" + r.Domain,
			"http://www." + r.Domain,
			"https://www." + r.Domain,
			"https://" + r.Domain,
		}
	}

	for _, u := range urls {
		result := probe(u)
		isHTTPS := strings.HasPrefix(u, "https")

		if result.Error != nil {
			errMsg := describeError(result.Error)
			if !r.SkipRedirect {
				redirectStatus = FAIL
			}
			if isHTTPS {
				httpsStatus = FAIL
			}
			if isHTTPS || !r.SkipRedirect {
				r.Fail(fmt.Sprintf("%s — %s", u, errMsg))
			}
			continue
		}

		if isHTTPS && strings.HasPrefix(result.FinalURL, "http://") {
			redirectStatus = FAIL
			httpsStatus = FAIL
			r.Fail(fmt.Sprintf("%s — downgraded to %s (expected %s)", u, result.FinalURL, strings.Join(r.ExpectedHosts, ", ")))
		}

		if !r.SkipRedirect {
			finalHost := domain.ExtractHost(result.FinalURL)
			if result.StatusCode != 200 {
				redirectStatus = FAIL
				r.Fail(fmt.Sprintf("%s — expected status 200, got %d (expected %s)", u, result.StatusCode, strings.Join(r.ExpectedHosts, ", ")))
			} else if !slices.Contains(r.ExpectedHosts, finalHost) {
				redirectStatus = FAIL
				r.Fail(fmt.Sprintf("%s — redirected to %s (expected %s)", u, result.FinalURL, strings.Join(r.ExpectedHosts, ", ")))
			} else if !slices.Contains(r.ExpectedHosts, r.Domain) {
				r.Verbosef("\033[32mPASS\033[0m  %s — forwarded to expected host\n", u)
			} else {
				r.Verbosef("\033[32mPASS\033[0m  %s\n", u)
			}
		}

		r.Verbosef("      → final: %s\n", result.FinalURL)
		r.Verbosef("      → status: %d\n", result.StatusCode)
		r.Verbosef("      → redirects: %d\n", result.Redirects)
		r.Verbosef("      → total: %.3fs\n", result.TotalTime.Seconds())

		if result.TotalTime.Seconds() > 8.0 {
			responseStatus = FAIL
			r.Fail(fmt.Sprintf("%s — response too slow: %.3fs", u, result.TotalTime.Seconds()))
		} else if result.TotalTime.Seconds() > 3.0 {
			responseStatus = WARN
			r.Warn(fmt.Sprintf("%s — response slow: %.3fs", u, result.TotalTime.Seconds()))
		}

		ms := int(result.TotalTime.Seconds() * 1000)
		responseMs = &ms
	}

	return redirectStatus, httpsStatus, responseStatus, responseMs
}

type ForwardingProbeResult struct {
	StatusCode int
	FinalURL   string
	Error      error
}

func (r *Runner) ProbeURLForForwarding(targetURL string) ForwardingProbeResult {
	client := r.httpClient()

	resp, err := client.Get(targetURL)
	if err != nil {
		return ForwardingProbeResult{Error: err}
	}
	defer resp.Body.Close()

	return ForwardingProbeResult{
		StatusCode: resp.StatusCode,
		FinalURL:   resp.Request.URL.String(),
	}
}

func describeError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return "DNS resolution failed"
	case strings.Contains(msg, "connection refused"):
		return "TCP connection refused"
	case strings.Contains(msg, "timeout"):
		return "request timed out"
	case strings.Contains(msg, "tls"):
		return "TLS/SSL handshake error"
	case strings.Contains(msg, "too many redirects"):
		return "too many redirects or redirect loop"
	case strings.Contains(msg, "EOF"):
		return "server returned empty response"
	default:
		return sanitizeTerminalText(msg)
	}
}

func sanitizeTerminalText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 0x20 && r != 0x7f {
			return r
		}
		return '\uFFFD'
	}, s)
}

func (r *Runner) CheckContent() {
	r.Verbosef("\n\033[1m== Canonical Page Content ==\033[0m\n")

	client := r.httpClient()

	expectedURL := "https://" + r.ExpectedHosts[0] + "/"

	start := time.Now()
	resp, err := client.Get(expectedURL)
	elapsed := time.Since(start)

	if err != nil {
		errMsg := describeError(err)
		r.Fail(fmt.Sprintf("%s — could not download content: %s", expectedURL, errMsg))
		return
	}
	defer resp.Body.Close()

	body, err := readLimitedBody(resp.Body)
	if err != nil {
		r.Fail(fmt.Sprintf("%s — error reading response body", expectedURL))
		return
	}

	r.Verbosef("      → content-type: %s\n", resp.Header.Get("Content-Type"))
	r.Verbosef("      → downloaded: %d bytes\n", len(body))

	if resp.StatusCode == 200 {
		r.Verbosef("\033[32mPASS\033[0m  canonical page returns 200\n")
	} else {
		r.Fail(fmt.Sprintf("canonical page — expected 200, got %d", resp.StatusCode))
	}

	finalHost := domain.ExtractHost(resp.Request.URL.String())
	if resp.Request.URL.String() == expectedURL || slices.Contains(r.ExpectedHosts, finalHost) {
		r.Verbosef("\033[32mPASS\033[0m  canonical page at expected host\n")
	} else {
		r.Fail(fmt.Sprintf("canonical page redirected to unexpected URL: %s", resp.Request.URL.String()))
	}

	if len(body) > 0 {
		r.Verbosef("\033[32mPASS\033[0m  response body is not empty\n")
	} else {
		r.Fail("response body is empty")
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/xhtml+xml") {
		r.Verbosef("\033[32mPASS\033[0m  Content-Type is HTML\n")
	} else {
		r.Fail(fmt.Sprintf("unexpected Content-Type: %s", ct))
	}

	bodyStr := string(body)
	if strings.Contains(strings.ToLower(bodyStr), "<!doctype html") ||
		strings.Contains(strings.ToLower(bodyStr), "<html") {
		r.Verbosef("\033[32mPASS\033[0m  response contains valid HTML document structure\n")
	} else {
		r.Fail("HTML document structure not found in response")
	}

	title := extractTitle(bodyStr)
	if title != "" {
		r.Verbosef("\033[32mPASS\033[0m  HTML title found\n")
		r.Verbosef("      → %s\n", title)
	} else {
		r.Warn("HTML title not found")
	}

	checkErrorPatterns(bodyStr, r)
	checkParkedPatterns(bodyStr, r)

	if strings.Contains(strings.ToLower(bodyStr), "coming soon") ||
		strings.Contains(strings.ToLower(bodyStr), "under construction") ||
		strings.Contains(strings.ToLower(bodyStr), "website is under maintenance") {
		r.Warn("page may be under maintenance or coming soon")
	}

	_ = elapsed
}

func extractTitle(html string) string {
	lower := strings.ToLower(html)
	startIdx := strings.Index(lower, "<title")
	if startIdx == -1 {
		return ""
	}

	gtIdx := strings.Index(html[startIdx:], ">")
	if gtIdx == -1 {
		return ""
	}

	contentStart := startIdx + gtIdx + 1
	endIdx := strings.Index(strings.ToLower(html[contentStart:]), "</title>")
	if endIdx == -1 {
		return ""
	}

	title := strings.TrimSpace(html[contentStart : contentStart+endIdx])
	return title
}

func checkErrorPatterns(body string, r *Runner) {
	patterns := []string{
		"Fatal error",
		"Parse error",
		"Uncaught Error",
		"Uncaught Exception",
		"Error establishing a database connection",
		"There has been a critical error on this website",
		"The site is experiencing technical difficulties",
		"WordPress database error",
		"Allowed memory size of",
		"Call to undefined function",
		"Call to undefined method",
		"502 Bad Gateway",
		"503 Service Unavailable",
		"504 Gateway Time-out",
		"Internal Server Error",
	}

	lower := strings.ToLower(body)
	found := []string{}

	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			found = append(found, p)
		}
	}

	if len(found) > 0 {
		r.Fail("application, WordPress, PHP, or server error text found in HTML")
		for i := 0; i < len(found) && i < 5; i++ {
			r.Verbosef("      → %s\n", found[i])
		}
	} else {
		r.Verbosef("\033[32mPASS\033[0m  no known WordPress/PHP/server error text found\n")
	}
}

func checkParkedPatterns(body string, r *Runner) {
	patterns := []string{
		"domain is parked",
		"domain has been parked",
		"this domain is parked",
		"buy this domain",
		"domain for sale",
		"this domain may be for sale",
		"sedo domain parking",
		"afternic",
		"dan.com",
		"hugedomains",
		"bodis",
		"parkingcrew",
		"namecheap parking page",
		"godaddy domain parking",
	}

	lower := strings.ToLower(body)
	found := []string{}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			found = append(found, p)
		}
	}

	if len(found) > 0 {
		r.Fail("domain may be parked")
		for i := 0; i < len(found) && i < 5; i++ {
			r.Verbosef("      → %s\n", found[i])
		}
	} else {
		r.Verbosef("\033[32mPASS\033[0m  no known parked domain page signature found\n")
	}
}

func (r *Runner) CheckLLMs() {
	r.Verbosef("\n\033[1m== llms.txt ==\033[0m\n")

	client := r.httpClient()

	r.checkLLMsWithClient(client)
}

func (r *Runner) checkLLMsWithClient(client *http.Client) {
	u := "https://" + r.Domain + "/llms.txt"

	resp, err := client.Get(u)
	if err != nil {
		errMsg := describeError(err)
		r.Warn(fmt.Sprintf("/llms.txt — %s", errMsg))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		r.Warn(fmt.Sprintf("/llms.txt — expected 200, got %d", resp.StatusCode))
		return
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "text/html") {
		r.Warn("/llms.txt — returned HTML instead of a text file (possible soft 404)")
		return
	}

	r.Verbosef("\033[32mPASS\033[0m  /llms.txt\n")
	r.Verbosef("      → %s\n", resp.Request.URL.String())
	r.Verbosef("      → content-type: %s\n", resp.Header.Get("Content-Type"))
}
