package output

import (
	"fmt"
	"io"

	"github.com/atillalab/site-health/internal/check"
)

func RenderDashboard(w io.Writer, report *check.Report) {
	fmt.Fprintf(w, "%sSite Health Check%s\n", Bold, Reset)
	fmt.Fprintf(w, "Domain: %s\n", report.Domain)

	if report.ExpectedHost != "" {
		fmt.Fprintf(w, "Expected Host: %s\n", report.ExpectedHost)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%sSITE HEALTH%s\n", Bold, Reset)
	fmt.Fprintln(w, "───────────")
	fmt.Fprintf(w, "● %s\n", report.Domain)

	if report.Checks.DNS != nil {
		renderSimpleRow(w, "DNS", report.Checks.DNS.Status.String())
	}

	if report.Checks.HTTPS != nil {
		renderSimpleRow(w, "HTTPS", report.Checks.HTTPS.Status.String())
	}

	if report.Checks.SSL != nil {
		if report.Checks.SSL.DaysRemaining != nil {
			renderValueRow(w, "SSL", fmt.Sprintf("%d days", *report.Checks.SSL.DaysRemaining), report.Checks.SSL.Status.String())
		} else {
			renderSimpleRow(w, "SSL", report.Checks.SSL.Status.String())
		}
	}

	if report.Checks.DomainRegistration != nil {
		renderValueRow(w, "Domain Reg", domainRegistrationValue(report.Checks.DomainRegistration), report.Checks.DomainRegistration.Status.String())
	}

	if report.Checks.Redirect != nil {
		renderSimpleRow(w, "Redirect", report.Checks.Redirect.Status.String())
	}

	if report.Checks.Response != nil {
		if report.Checks.Response.Ms != nil {
			renderValueRow(w, "Response", fmt.Sprintf("%d ms", *report.Checks.Response.Ms), report.Checks.Response.Status.String())
		} else {
			renderSimpleRow(w, "Response", "—")
		}
	}

	if report.Checks.Mail != nil {
		renderSimpleRow(w, "Mail", report.Checks.Mail.Status.String())
	}

	fmt.Fprintf(w, "Status: %s%s%s\n", StatusColor(report.Summary.Status), report.Summary.Status, Reset)

	renderIssues(w, report.Issues)
}

func RenderMailDashboard(w io.Writer, report *check.Report) {
	fmt.Fprintf(w, "%sMAIL HEALTH%s\n", Bold, Reset)
	fmt.Fprintln(w, "───────────")
	fmt.Fprintf(w, "● %s\n", report.Domain)

	if report.Checks.Mail != nil {
		if report.Checks.Mail.MX != nil {
			renderSimpleRow(w, "MX", report.Checks.Mail.MX.Status.String())
		}
		if report.Checks.Mail.SPF != nil {
			renderSimpleRow(w, "SPF", report.Checks.Mail.SPF.Status.String())
		}
		if report.Checks.Mail.DMARC != nil {
			renderSimpleRow(w, "DMARC", report.Checks.Mail.DMARC.Status.String())
		}
	}

	fmt.Fprintf(w, "Status: %s%s%s\n", StatusColor(report.Summary.Status), report.Summary.Status, Reset)

	renderIssues(w, report.Issues)
}

func renderSimpleRow(w io.Writer, label, status string) {
	fmt.Fprintf(w, "  %-12s %s\n", label, FormatStatusToken(status))
}

func renderValueRow(w io.Writer, label, value, status string) {
	switch status {
	case "WARN":
		fmt.Fprintf(w, "  %-12s %s    %s⚠ WARN%s\n", label, value, Yellow, Reset)
	case "FAIL":
		fmt.Fprintf(w, "  %-12s %s    %s✖ FAIL%s\n", label, value, Red, Reset)
	default:
		fmt.Fprintf(w, "  %-12s %s\n", label, value)
	}
}

func domainRegistrationValue(result *check.DomainRegistrationResult) string {
	expiryDate := domainRegistrationExpiryDate(result.ExpiresAt)
	if result.DaysRemaining != nil && expiryDate != "" {
		return fmt.Sprintf("%d days (%s)", *result.DaysRemaining, expiryDate)
	}
	if result.DaysRemaining != nil {
		return fmt.Sprintf("%d days", *result.DaysRemaining)
	}
	if expiryDate != "" {
		return expiryDate
	}
	return "Unknown"
}

func domainRegistrationExpiryDate(expiresAt string) string {
	if expiresAt == "" {
		return ""
	}

	t, err := check.ParseExpiryDate(expiresAt)
	if err != nil {
		return ""
	}
	return t.Format("02 Jan 2006")
}

func renderIssues(w io.Writer, issues []check.Issue) {
	if len(issues) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Issues:")

	maxIssues := 10
	for i, issue := range issues {
		if i >= maxIssues {
			fmt.Fprintf(w, "  ...  %d more; run with --verbose for full diagnostics\n", len(issues)-i)
			break
		}
		fmt.Fprintf(w, "  %s%-4s%s  %s\n", StatusColor(issue.Level), issue.Level, Reset, issue.Message)
	}
}
