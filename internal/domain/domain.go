package domain

import (
	"errors"
	"net/url"
	"strings"
)

var ErrInvalidURL = errors.New("invalid URL: missing scheme or host")

func Normalize(raw string) string {
	s := strings.ToLower(raw)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimRight(s, ".")
	s = strings.SplitN(s, "/", 2)[0]
	s = strings.SplitN(s, ":", 2)[0]
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
