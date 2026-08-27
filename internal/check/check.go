package check

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/atillalab/site-health/internal/domain"
	"github.com/atillalab/site-health/internal/version"
)

type Status int

const (
	OK Status = iota
	WARN
	FAIL
	SKIP
)

func (s Status) String() string {
	switch s {
	case WARN:
		return "WARN"
	case FAIL:
		return "FAIL"
	case SKIP:
		return "SKIP"
	default:
		return "OK"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

func Escalate(current, incoming Status) Status {
	if current == SKIP {
		return incoming
	}
	if incoming == SKIP {
		return current
	}
	if incoming > current {
		return incoming
	}
	return current
}

type Issue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Report struct {
	Tool         string     `json:"tool"`
	Version      string     `json:"version"`
	Domain       string     `json:"domain"`
	Mode         string     `json:"mode"`
	ExpectedHost string     `json:"expected_host,omitempty"`
	Forwarding   Forwarding `json:"forwarding,omitempty"`
	Checks       Checks     `json:"checks"`
	Issues       []Issue    `json:"issues"`
	Summary      Summary    `json:"summary"`
}

type Forwarding struct {
	AutoDetected bool     `json:"auto_detected"`
	Ambiguous    bool     `json:"ambiguous"`
	HintHost     string   `json:"hint_host,omitempty"`
	Candidates   []string `json:"candidates"`
}

type Checks struct {
	DNS                *DNSResult                `json:"dns,omitempty"`
	HTTPS              *SimpleResult             `json:"https,omitempty"`
	SSL                *SSLResult                `json:"ssl,omitempty"`
	Redirect           *SimpleResult             `json:"redirect,omitempty"`
	Response           *ResponseResult           `json:"response,omitempty"`
	DomainRegistration *DomainRegistrationResult `json:"domain_registration,omitempty"`
	Mail               *MailResult               `json:"mail,omitempty"`
}

type Summary struct {
	Failures int    `json:"failures"`
	Warnings int    `json:"warnings"`
	Status   string `json:"status"`
}

type SimpleResult struct {
	Status Status `json:"status"`
}

type DNSResult struct {
	Status Status   `json:"status"`
	A      []string `json:"a"`
	AAAA   []string `json:"aaaa"`
}

type SSLResult struct {
	Status        Status `json:"status"`
	DaysRemaining *int   `json:"days_remaining,omitempty"`
	Subject       string `json:"subject,omitempty"`
	Issuer        string `json:"issuer,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

type ResponseResult struct {
	Status Status `json:"status"`
	Ms     *int   `json:"ms,omitempty"`
}

type DomainRegistrationResult struct {
	Status        Status `json:"status"`
	Registrar     string `json:"registrar,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	DaysRemaining *int   `json:"days_remaining,omitempty"`
}

type MailResult struct {
	Status Status       `json:"status"`
	MX     *MXResult    `json:"mx,omitempty"`
	SPF    *SPFResult   `json:"spf,omitempty"`
	DMARC  *DMARCResult `json:"dmarc,omitempty"`
}

type MXResult struct {
	Status  Status   `json:"status"`
	Records []string `json:"records"`
}

type SPFResult struct {
	Status  Status   `json:"status"`
	Records []string `json:"records"`
}

type DMARCResult struct {
	Status  Status   `json:"status"`
	Records []string `json:"records"`
}

type MailChecks struct {
	MX    bool
	SPF   bool
	DMARC bool
}

func DefaultMailChecks() MailChecks {
	return MailChecks{MX: true, SPF: true, DMARC: true}
}

func (m MailChecks) EnabledCount() int {
	count := 0
	if m.MX {
		count++
	}
	if m.SPF {
		count++
	}
	if m.DMARC {
		count++
	}
	return count
}

type Runner struct {
	Domain       string
	ExpectedHost string
	MailOnly     bool
	Verbose      bool
	SkipMail     bool
	MailChecks   MailChecks
	SkipLLMs     bool
	SkipRedirect bool

	ForwardingAutoDetected bool
	ForwardingAmbiguous    bool
	ForwardingHintHost     string
	ForwardingCandidates   []string

	mu        sync.Mutex
	issues    []Issue
	failCount int
	warnCount int
}

func (r *Runner) Fail(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.issues = append(r.issues, Issue{Level: "FAIL", Message: msg})
	r.failCount++
}

func (r *Runner) Warn(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.issues = append(r.issues, Issue{Level: "WARN", Message: msg})
	r.warnCount++
}

func (r *Runner) Verbosef(format string, args ...any) {
	if r.Verbose {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// effectiveMailDomain returns the domain against which mail records (MX,
// SPF, DMARC) should be evaluated. When running in mail-only mode against a
// subdomain, the apex domain is used because mail policy normally lives there.
func (r *Runner) effectiveMailDomain() string {
	if r.MailOnly && domain.IsSubdomain(r.Domain) {
		return domain.ApexDomain(r.Domain)
	}
	return r.Domain
}

func (r *Runner) OverallStatus() string {
	if r.failCount > 0 {
		return "UNHEALTHY"
	}
	if r.warnCount > 0 {
		return "WARNING"
	}
	return "HEALTHY"
}

func (r *Runner) shouldRunMailChecks() bool {
	return !r.SkipMail && !domain.IsSubdomain(r.Domain)
}

func (r *Runner) enabledMailChecks() MailChecks {
	if r.MailChecks.EnabledCount() == 0 {
		return DefaultMailChecks()
	}
	return r.MailChecks
}

func (r *Runner) buildReport(mode string) *Report {
	return &Report{
		Tool:         "site-health",
		Version:      version.Version,
		Domain:       r.Domain,
		Mode:         mode,
		ExpectedHost: r.ExpectedHost,
		Forwarding: Forwarding{
			AutoDetected: r.ForwardingAutoDetected,
			Ambiguous:    r.ForwardingAmbiguous,
			HintHost:     r.ForwardingHintHost,
			Candidates:   r.ForwardingCandidates,
		},
		Issues: r.issues,
		Summary: Summary{
			Failures: r.failCount,
			Warnings: r.warnCount,
			Status:   r.OverallStatus(),
		},
	}
}

func (r *Runner) RunChecks(ctx context.Context) *Report {
	r.mu.Lock()
	r.issues = nil
	r.failCount = 0
	r.warnCount = 0
	r.mu.Unlock()

	if r.MailOnly {
		mailResult := r.CheckMail()
		report := r.buildReport("mail")
		report.Checks.Mail = mailResult
		if report.Forwarding.Candidates == nil {
			report.Forwarding.Candidates = []string{}
		}
		return report
	}

	var (
		wg              sync.WaitGroup
		dnsResult       *DNSResult
		sslResult       *SSLResult
		domainRegResult *DomainRegistrationResult
		mailResult      *MailResult
		redirectStatus  Status
		httpsStatus     Status
		responseStatus  Status
		responseMs      *int
	)

	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	run(func() { dnsResult = r.CheckDNS() })
	run(func() {
		redirectStatus, httpsStatus, responseStatus, responseMs = r.CheckHTTP()
	})
	run(func() { sslResult = r.CheckSSL() })
	run(func() { domainRegResult = r.CheckDomainRegistration() })
	if r.shouldRunMailChecks() {
		run(func() { mailResult = r.CheckMail() })
	} else if r.SkipMail {
		r.Verbosef("\n\033[1m== Mail ==\033[0m\n")
		r.Verbosef("\033[36mINFO\033[0m  mail checks skipped by --skip-mail\n")
	} else {
		r.Verbosef("\n\033[1m== Mail ==\033[0m\n")
		r.Verbosef("\033[36mINFO\033[0m  subdomain detected; skipping mail checks in site mode\n")
	}
	run(func() { r.CheckContent() })
	if !r.SkipLLMs {
		run(func() { r.CheckLLMs() })
	}

	wg.Wait()

	redirectResult := &SimpleResult{Status: redirectStatus}
	httpsResult := &SimpleResult{Status: httpsStatus}
	responseResult := &ResponseResult{Status: responseStatus, Ms: responseMs}

	report := r.buildReport("site")
	report.Checks = Checks{
		DNS:                dnsResult,
		HTTPS:              httpsResult,
		SSL:                sslResult,
		Redirect:           redirectResult,
		Response:           responseResult,
		DomainRegistration: domainRegResult,
		Mail:               mailResult,
	}

	if report.Forwarding.Candidates == nil {
		report.Forwarding.Candidates = []string{}
	}

	return report
}
