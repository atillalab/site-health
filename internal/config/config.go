package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Source holds configuration values that can come from a config file or
// environment variables. Pointer fields represent optional values: nil means
// the setting was not provided and the hard-coded default should be used.
type Source struct {
	Verbose        *bool    `json:"verbose,omitempty"`
	SkipRedirect   *bool    `json:"skip_redirect,omitempty"`
	SkipMail       *bool    `json:"skip_mail,omitempty"`
	SkipLLMs       *bool    `json:"skip_llms_txt,omitempty"`
	Format         *string  `json:"format,omitempty"`
	ExpectedHosts  []string `json:"expected_hosts,omitempty"`
	MailChecks     []string `json:"mail_checks,omitempty"`
	SkipMailChecks []string `json:"skip_mail_checks,omitempty"`
}

// EnvVar describes a supported SITE_HEALTH_* environment variable.
type EnvVar struct {
	Name string
	Kind string // "bool" or "string"
}

// SupportedEnvVars returns metadata for all environment variables recognized
// by FromEnv. The order is stable and matches the fields in Source.
func SupportedEnvVars() []EnvVar {
	return []EnvVar{
		{Name: "SITE_HEALTH_VERBOSE", Kind: "bool"},
		{Name: "SITE_HEALTH_SKIP_REDIRECT", Kind: "bool"},
		{Name: "SITE_HEALTH_SKIP_MAIL", Kind: "bool"},
		{Name: "SITE_HEALTH_SKIP_LLMS_TXT", Kind: "bool"},
		{Name: "SITE_HEALTH_FORMAT", Kind: "string"},
		{Name: "SITE_HEALTH_EXPECTED_HOSTS", Kind: "string"},
		{Name: "SITE_HEALTH_MAIL_CHECKS", Kind: "string"},
		{Name: "SITE_HEALTH_SKIP_MAIL_CHECKS", Kind: "string"},
	}
}

// DefaultPath returns the default config file path using the XDG Base Directory
// Specification. If XDG_CONFIG_HOME is unset, it falls back to ~/.config.
func DefaultPath() string {
	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cfgDir = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgDir, "site-health", "config.json")
}

// Load reads configuration from path. A missing file is not an error and
// returns an empty Source. Malformed JSON is returned as an error.
func Load(path string) (*Source, error) {
	if path == "" {
		return &Source{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Source{}, nil
		}
		return nil, err
	}
	var src Source
	if err := json.Unmarshal(data, &src); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &src, nil
}

// FromEnv reads supported SITE_HEALTH_* environment variables into a Source.
// Boolean variables accept values parseable by strconv.ParseBool.
func FromEnv() *Source {
	var s Source
	for _, ev := range SupportedEnvVars() {
		v, ok := os.LookupEnv(ev.Name)
		if !ok {
			continue
		}
		switch ev.Name {
		case "SITE_HEALTH_VERBOSE":
			if b, err := strconv.ParseBool(v); err == nil {
				s.Verbose = &b
			}
		case "SITE_HEALTH_SKIP_REDIRECT":
			if b, err := strconv.ParseBool(v); err == nil {
				s.SkipRedirect = &b
			}
		case "SITE_HEALTH_SKIP_MAIL":
			if b, err := strconv.ParseBool(v); err == nil {
				s.SkipMail = &b
			}
		case "SITE_HEALTH_SKIP_LLMS_TXT":
			if b, err := strconv.ParseBool(v); err == nil {
				s.SkipLLMs = &b
			}
		case "SITE_HEALTH_FORMAT":
			v = strings.TrimSpace(v)
			if v != "" {
				s.Format = &v
			}
		case "SITE_HEALTH_EXPECTED_HOSTS":
			s.ExpectedHosts = splitAndTrim(v)
		case "SITE_HEALTH_MAIL_CHECKS":
			s.MailChecks = splitAndTrim(v)
		case "SITE_HEALTH_SKIP_MAIL_CHECKS":
			s.SkipMailChecks = splitAndTrim(v)
		}
	}
	return &s
}

func splitAndTrim(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, strings.ToLower(trimmed))
		}
	}
	return out
}
