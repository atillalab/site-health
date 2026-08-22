package check

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/atillalab/site-health/internal/domain"
)

func (r *Runner) CheckDNS() *DNSResult {
	result := &DNSResult{Status: OK}

	r.Verbosef("\n\033[1m== DNS ==\033[0m\n")

	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), r.Domain)
	if err != nil || len(ips) == 0 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("DNS %s — no A record found", r.Domain))
	} else {
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				result.A = append(result.A, ip.IP.String())
			} else {
				result.AAAA = append(result.AAAA, ip.IP.String())
			}
		}
		if len(result.A) > 0 {
			r.Verbosef("\033[32mPASS\033[0m  A %s\n", r.Domain)
			for _, a := range result.A {
				r.Verbosef("      → %s\n", a)
			}
		} else {
			result.Status = FAIL
			r.Fail(fmt.Sprintf("DNS %s — no IPv4 address found", r.Domain))
		}
		if len(result.AAAA) > 0 {
			r.Verbosef("\033[32mPASS\033[0m  AAAA %s\n", r.Domain)
			for _, aaaa := range result.AAAA {
				r.Verbosef("      → %s\n", aaaa)
			}
		} else {
			r.Verbosef("\033[36mINFO\033[0m  AAAA %s — no IPv6 record found\n", r.Domain)
		}
	}

	if !domain.IsSubdomain(r.Domain) {
		www := "www." + r.Domain
		wwwCNAME, errCNAME := net.LookupCNAME(www)
		wwwIPs, errIP := net.DefaultResolver.LookupIPAddr(context.Background(), www)

		if errCNAME == nil || errIP == nil {
			r.Verbosef("\033[32mPASS\033[0m  www.%s resolves\n", r.Domain)
			if errCNAME == nil {
				r.Verbosef("      → %s\n", wwwCNAME)
			}
		} else {
			result.Status = FAIL
			r.Fail(fmt.Sprintf("DNS www.%s — no DNS record found", r.Domain))
		}
		_ = wwwIPs
	}

	return result
}

func (r *Runner) CheckMX() MXResult {
	result := MXResult{Status: OK}

	r.Verbosef("\n\033[1m== MX ==\033[0m\n")

	mailDomain := r.effectiveMailDomain()
	if mailDomain != r.Domain {
		r.Verbosef("\033[36mINFO\033[0m  subdomain detected; checking MX on apex domain %s\n", mailDomain)
	}

	mxRecords, err := net.LookupMX(mailDomain)
	if err != nil || len(mxRecords) == 0 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("MX %s — no MX record found", mailDomain))
		return result
	}

	nullMX := 0
	for _, mx := range mxRecords {
		record := fmt.Sprintf("%d %s", mx.Pref, mx.Host)
		result.Records = append(result.Records, record)
		if mx.Pref == 0 && mx.Host == "." {
			nullMX++
		}
	}

	if nullMX == 1 && len(mxRecords) == 1 {
		r.Verbosef("\033[36mINFO\033[0m  Null MX detected — this domain explicitly does not accept email\n")
	} else if nullMX > 0 {
		r.Verbosef("\033[36mINFO\033[0m  Null MX record detected alongside other MX records\n")
	}

	r.Verbosef("\033[32mPASS\033[0m  MX %s\n", mailDomain)
	for _, rec := range result.Records {
		r.Verbosef("      → %s\n", rec)
	}

	return result
}

func (r *Runner) CheckSPF() SPFResult {
	result := SPFResult{Status: OK}

	r.Verbosef("\n\033[1m== SPF ==\033[0m\n")

	mailDomain := r.effectiveMailDomain()
	if mailDomain != r.Domain {
		r.Verbosef("\033[36mINFO\033[0m  subdomain detected; checking SPF on apex domain %s\n", mailDomain)
	}

	txtRecords, err := net.LookupTXT(mailDomain)
	if err != nil {
		txtRecords = nil
	}

	for _, txt := range txtRecords {
		normalized := strings.ReplaceAll(txt, "\"", "")
		normalized = strings.TrimSpace(normalized)
		if strings.HasPrefix(strings.ToLower(normalized), "v=spf1") {
			result.Records = append(result.Records, normalized)
		}
	}

	spfCount := len(result.Records)

	if spfCount == 0 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("SPF %s — no SPF TXT record found", mailDomain))
	} else if spfCount > 1 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("SPF %s — multiple SPF TXT records found", mailDomain))
		for _, rec := range result.Records {
			r.Verbosef("      → %s\n", rec)
		}
	} else {
		r.Verbosef("\033[32mPASS\033[0m  SPF %s\n", mailDomain)
		for _, rec := range result.Records {
			r.Verbosef("      → %s\n", rec)
		}
	}

	return result
}

func (r *Runner) CheckDMARC() DMARCResult {
	result := DMARCResult{Status: OK}

	r.Verbosef("\n\033[1m== DMARC ==\033[0m\n")

	mailDomain := r.effectiveMailDomain()
	if mailDomain != r.Domain {
		r.Verbosef("\033[36mINFO\033[0m  subdomain detected; checking DMARC on apex domain %s\n", mailDomain)
	}

	dmarcDomain := "_dmarc." + mailDomain
	txtRecords, err := net.LookupTXT(dmarcDomain)
	if err != nil {
		txtRecords = nil
	}

	for _, txt := range txtRecords {
		normalized := strings.ReplaceAll(txt, "\"", "")
		normalized = strings.TrimSpace(normalized)
		if strings.HasPrefix(strings.ToLower(normalized), "v=dmarc1") {
			result.Records = append(result.Records, normalized)
		}
	}

	dmarcCount := len(result.Records)

	if dmarcCount == 0 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("DMARC %s — no DMARC TXT record found", dmarcDomain))
		r.Verbosef("      → Add a DMARC record to prevent spoofed emails from @%s\n", mailDomain)
	} else if dmarcCount > 1 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("DMARC %s — multiple DMARC TXT records found", dmarcDomain))
		for _, rec := range result.Records {
			r.Verbosef("      → %s\n", rec)
		}
	} else {
		record := result.Records[0]
		lower := strings.ToLower(record)

		policy := ""
		for _, part := range strings.Split(lower, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "p=") {
				policy = strings.TrimPrefix(part, "p=")
				break
			}
		}

		switch policy {
		case "none":
			result.Status = WARN
			r.Warn(fmt.Sprintf("DMARC %s — policy p=none, monitoring only", dmarcDomain))
		case "quarantine", "reject":
			r.Verbosef("\033[32mPASS\033[0m  DMARC %s — policy p=%s\n", dmarcDomain, policy)
		default:
			result.Status = FAIL
			r.Fail(fmt.Sprintf("DMARC %s — no valid p= policy found", dmarcDomain))
		}

		for _, rec := range result.Records {
			r.Verbosef("      → %s\n", rec)
		}
	}

	return result
}

func (r *Runner) CheckMail() *MailResult {
	mx := r.CheckMX()
	spf := r.CheckSPF()
	dmarc := r.CheckDMARC()

	status := OK
	status = Escalate(status, mx.Status)
	status = Escalate(status, spf.Status)
	status = Escalate(status, dmarc.Status)

	return &MailResult{
		Status: status,
		MX:     mx,
		SPF:    spf,
		DMARC:  dmarc,
	}
}
