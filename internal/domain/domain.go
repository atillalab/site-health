package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
)

var ErrInvalidURL = errors.New("invalid URL: missing scheme or host")

// ValidateWebTarget validates the deliberately narrow target syntax accepted
// by the local web UI. CLI normalization remains permissive for diagnostics.
func ValidateWebTarget(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) != raw {
		return "", errors.New("target must not be empty or contain surrounding whitespace")
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return "", errors.New("target contains control characters")
		}
	}
	host := raw
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Port() != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return "", errors.New("target must be a bare hostname or an http(s) URL without credentials, ports, or paths")
		}
		host = u.Hostname()
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || strings.ContainsAny(host, "/:@") {
		return "", errors.New("invalid hostname")
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	if strings.Contains(host, ":") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return "", errors.New("invalid hostname")
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid hostname")
		}
		for _, r := range label {
			if !(r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return "", fmt.Errorf("invalid hostname")
			}
		}
	}
	if len(host) > 253 {
		return "", errors.New("hostname is too long")
	}
	return host, nil
}

func Normalize(raw string) string {
	s := strings.ToLower(raw)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimRight(s, ".")
	s = strings.SplitN(s, "/", 2)[0]
	s = strings.SplitN(s, ":", 2)[0]
	s = strings.TrimPrefix(s, "www.")
	return s
}

func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())

	if scheme == "" || host == "" {
		return "", ErrInvalidURL
	}

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}

	return scheme + "://" + host + path, nil
}

func ExtractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ParseHost accepts either a bare host or an absolute URL and returns the
// normalized host. It is used for flags values that accept both forms, such
// as --expected-host.
func ParseHost(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(s, "://") {
		return ExtractHost(s)
	}
	return s
}

func IsSameSiteHost(host, domain string) bool {
	d := strings.ToLower(domain)
	apex := strings.TrimPrefix(d, "www.")

	host = strings.ToLower(host)

	return host == d ||
		host == "www."+d ||
		host == apex ||
		host == "www."+apex
}

// IsSubdomain reports whether domain is a host below an apex domain
// (e.g. "sub.example.com"). The synthetic "www.example.com" prefix is
// treated as an alias of the apex domain, not a subdomain.
func IsSubdomain(domain string) bool {
	d := strings.ToLower(strings.TrimRight(domain, "."))
	d = strings.TrimPrefix(d, "www.")
	parts := strings.Split(d, ".")
	return len(parts) > 2
}

// ApexDomain returns the registered apex domain for the given host using a
// simple last-two-label heuristic. It strips a leading "www." when present.
// Examples:
//   - "example.com" -> "example.com"
//   - "www.example.com" -> "example.com"
//   - "sub.example.com" -> "example.com"
//   - "a.b.example.co.uk" -> "example.co.uk" (best-effort)
func ApexDomain(domain string) string {
	d := strings.ToLower(strings.TrimRight(domain, "."))
	d = strings.TrimPrefix(d, "www.")
	parts := strings.Split(d, ".")
	if len(parts) < 2 {
		return d
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
