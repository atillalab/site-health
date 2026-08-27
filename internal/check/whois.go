package check

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/atillalab/site-health/internal/domain"
)

const whoisTimeout = 10 * time.Second

var whoisServers = map[string]string{
	"com":  "whois.verisign-grs.com",
	"net":  "whois.verisign-grs.com",
	"org":  "whois.pir.org",
	"info": "whois.afilias.net",
	"io":   "whois.nic.io",
	"co":   "whois.nic.co",
	"de":   "whois.denic.de",
	"uk":   "whois.nic.uk",
	"nl":   "whois.sidn.nl",
	"be":   "whois.dns.be",
	"fr":   "whois.nic.fr",
	"it":   "whois.nic.it",
	"es":   "whois.nic.es",
	"pt":   "whois.dns.pt",
	"ch":   "whois.nic.ch",
	"at":   "whois.nic.at",
	"pl":   "whois.dns.pl",
	"se":   "whois.iis.se",
	"no":   "whois.norid.no",
	"dk":   "whois.dk-hostmaster.dk",
	"fi":   "whois.fi",
	"cz":   "whois.nic.cz",
	"sk":   "whois.sk-nic.sk",
	"hu":   "whois.nic.hu",
	"ro":   "whois.rotld.ro",
	"bg":   "whois.register.bg",
	"hr":   "whois.dns.hr",
	"si":   "whois.arnes.si",
	"lt":   "whois.domreg.lt",
	"lv":   "whois.nic.lv",
	"ee":   "whois.eestihosting.ee",
	"ie":   "whois.weare.ie",
	"nz":   "whois.srs.net.nz",
	"au":   "whois.auda.org.au",
	"ca":   "whois.cira.ca",
	"jp":   "whois.jprs.jp",
	"kr":   "whois.kr",
	"cn":   "whois.cnnic.cn",
	"in":   "whois.inregistry.net",
	"br":   "whois.registro.br",
	"mx":   "whois.nic.mx",
	"cl":   "whois.nic.cl",
	"ar":   "whois.nic.ar",
	"za":   "whois.registry.net.za",
	"ru":   "whois.tcinet.ru",
	"ua":   "whois.ua",
	"tr":   "whois.nic.tr",
	"th":   "whois.thnic.co.th",
	"ph":   "whois.dot.ph",
	"sg":   "whois.sgnic.sg",
	"hk":   "whois.hkirc.hk",
	"tw":   "whois.twnic.net.tw",
	"my":   "whois.mynic.my",
	"id":   "whois.id",
	"vn":   "whois.vnnic.vn",
	"pk":   "whois.pknic.net.pk",
	"ke":   "whois.kenic.or.ke",
	"ng":   "whois.nic.net.ng",
	"eg":   "whois.ripe.net",
	"ae":   "whois.aedns.ae",
	"sa":   "whois.nic.sa",
	"il":   "whois.isoc.org.il",
}

var expiryFieldRegex = regexp.MustCompile(`(?i)^\s*(registry expiry date|registrar registration expiration date|expiration date|expiry date|paid-till|validity)\s*:\s*(.+)$`)
var registrarFieldRegex = regexp.MustCompile(`(?i)^\s*(registrar|registrar name|sponsoring registrar)\s*:\s*(.+)$`)

var expiryDateFormats = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02-Jan-2006",
	"Jan 2 15:04:05 2006 MST",
	"Jan  2 15:04:05 2006 MST",
	"02 Jan 2006",
	"2 Jan 2006",
	"2006/01/02",
	"01/02/2006",
}

func getTLDDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func ianaWhoisReferral(line string) string {
	key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(key)) {
	case "refer", "whois", "whois server":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func getAuthoritativeWhoisServer(tld string) (string, error) {
	conn, err := net.DialTimeout("tcp", "whois.iana.org:43", whoisTimeout)
	if err != nil {
		return "", fmt.Errorf("failed to connect to whois.iana.org: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(whoisTimeout))

	fmt.Fprintf(conn, "%s\r\n", tld)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if server := ianaWhoisReferral(line); server != "" {
			return server, nil
		}
	}

	if server, ok := whoisServers[tld]; ok {
		return server, nil
	}

	return "", fmt.Errorf("no WHOIS server found for TLD: %s", tld)
}

func queryWhois(server, domain string) (string, error) {
	conn, err := net.DialTimeout("tcp", server+":43", whoisTimeout)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", server, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(whoisTimeout))

	fmt.Fprintf(conn, "%s\r\n", domain)

	var sb strings.Builder
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func parseExpiryDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)

	for _, format := range expiryDateFormats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

func ParseExpiryDate(dateStr string) (time.Time, error) {
	return parseExpiryDate(dateStr)
}

func domainRegistrationStatus(daysRemaining int) Status {
	if daysRemaining <= 14 {
		return FAIL
	}
	if daysRemaining <= 60 {
		return WARN
	}
	return OK
}

func (r *Runner) CheckDomainRegistration() *DomainRegistrationResult {
	result := &DomainRegistrationResult{Status: OK}

	r.Verbosef("\n\033[1m== Domain Registration ==\033[0m\n")

	whoisDomain := r.Domain
	if domain.IsSubdomain(r.Domain) {
		whoisDomain = domain.ApexDomain(r.Domain)
		r.Verbosef("\033[36mINFO\033[0m  subdomain detected; checking registration of apex domain %s\n", whoisDomain)
	}

	tld := getTLDDomain(whoisDomain)
	if tld == "" {
		result.Status = FAIL
		r.Fail("could not determine TLD for domain")
		return result
	}

	server, err := getAuthoritativeWhoisServer(tld)
	if err != nil {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("domain registration lookup failed: %s", err))
		return result
	}

	output, err := queryWhois(server, whoisDomain)
	if err != nil {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("domain registration lookup failed: %s", err))
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if match := expiryFieldRegex.FindStringSubmatch(line); match != nil {
			dateStr := strings.TrimSpace(match[2])
			result.ExpiresAt = dateStr

			if t, err := parseExpiryDate(dateStr); err == nil {
				daysRemaining := int(time.Until(t).Hours() / 24)
				result.DaysRemaining = &daysRemaining

				switch domainRegistrationStatus(daysRemaining) {
				case FAIL:
					result.Status = FAIL
					if daysRemaining < 0 {
						r.Fail(fmt.Sprintf("domain registration expired %d days ago", -daysRemaining))
					} else {
						r.Fail(fmt.Sprintf("domain registration expires in %d days", daysRemaining))
					}
				case WARN:
					result.Status = WARN
					r.Warn(fmt.Sprintf("domain registration expires in %d days", daysRemaining))
				default:
					r.Verbosef("\033[32mPASS\033[0m  domain registration expires in %d days\n", daysRemaining)
				}
			} else {
				result.Status = WARN
				r.Warn(fmt.Sprintf("domain registration expiry date could not be parsed: %s", dateStr))
			}
		}

		if match := registrarFieldRegex.FindStringSubmatch(line); match != nil {
			result.Registrar = strings.TrimSpace(match[2])
		}
	}

	r.logDomainRegistrationDetails(result)

	if result.ExpiresAt == "" {
		result.Status = WARN
		r.Warn("domain registration expiry date could not be read")
	}

	return result
}

func (r *Runner) logDomainRegistrationDetails(result *DomainRegistrationResult) {
	if result.Registrar != "" {
		r.Verbosef("      → registrar: %s\n", result.Registrar)
	}

	if result.ExpiresAt != "" {
		r.Verbosef("      → expiry date: %s\n", result.ExpiresAt)
	}
}
