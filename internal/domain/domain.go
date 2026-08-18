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
