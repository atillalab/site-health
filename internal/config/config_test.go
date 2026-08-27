package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got := DefaultPath()
	want := filepath.Join("/tmp/xdg", "site-health", "config.json")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPathFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine user home: %v", err)
	}
	got := DefaultPath()
	want := filepath.Join(home, ".config", "site-health", "config.json")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	src, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load() missing file error = %v", err)
	}
	if src == nil {
		t.Fatal("Load() returned nil Source for missing file")
	}
	if !reflect.DeepEqual(src, &Source{}) {
		t.Errorf("Load() = %+v, want empty Source", src)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"verbose": true,
		"skip_redirect": true,
		"format": "json",
		"expected_hosts": ["example.com", "www.example.com"],
		"mail_checks": ["mx", "spf"]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	src, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if src.Verbose == nil || !*src.Verbose {
		t.Errorf("verbose = %v, want true", src.Verbose)
	}
	if src.SkipRedirect == nil || !*src.SkipRedirect {
		t.Errorf("skip_redirect = %v, want true", src.SkipRedirect)
	}
	if src.Format == nil || *src.Format != "json" {
		t.Errorf("format = %v, want json", src.Format)
	}
	if !reflect.DeepEqual(src.ExpectedHosts, []string{"example.com", "www.example.com"}) {
		t.Errorf("expected_hosts = %v, want [example.com www.example.com]", src.ExpectedHosts)
	}
	if !reflect.DeepEqual(src.MailChecks, []string{"mx", "spf"}) {
		t.Errorf("mail_checks = %v, want [mx spf]", src.MailChecks)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid JSON")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SITE_HEALTH_VERBOSE", "true")
	t.Setenv("SITE_HEALTH_SKIP_REDIRECT", "1")
	t.Setenv("SITE_HEALTH_SKIP_MAIL", "TRUE")
	t.Setenv("SITE_HEALTH_SKIP_LLMS_TXT", "true")
	t.Setenv("SITE_HEALTH_WHOIS", "true")
	t.Setenv("SITE_HEALTH_FORMAT", "json")
	t.Setenv("SITE_HEALTH_EXPECTED_HOSTS", "example.com, www.example.com")
	t.Setenv("SITE_HEALTH_MAIL_CHECKS", "mx, dmarc")
	t.Setenv("SITE_HEALTH_SKIP_MAIL_CHECKS", "spf")

	src := FromEnv()

	if src.Verbose == nil || !*src.Verbose {
		t.Errorf("verbose = %v, want true", src.Verbose)
	}
	if src.SkipRedirect == nil || !*src.SkipRedirect {
		t.Errorf("skip_redirect = %v, want true", src.SkipRedirect)
	}
	if src.SkipMail == nil || !*src.SkipMail {
		t.Errorf("skip_mail = %v, want true", src.SkipMail)
	}
	if src.SkipLLMs == nil || !*src.SkipLLMs {
		t.Errorf("skip_llms_txt = %v, want true", src.SkipLLMs)
	}
	if src.WhoisOnly == nil || !*src.WhoisOnly {
		t.Errorf("whois = %v, want true", src.WhoisOnly)
	}
	if src.Format == nil || *src.Format != "json" {
		t.Errorf("format = %v, want json", src.Format)
	}
	if !reflect.DeepEqual(src.ExpectedHosts, []string{"example.com", "www.example.com"}) {
		t.Errorf("expected_hosts = %v, want [example.com www.example.com]", src.ExpectedHosts)
	}
	if !reflect.DeepEqual(src.MailChecks, []string{"mx", "dmarc"}) {
		t.Errorf("mail_checks = %v, want [mx dmarc]", src.MailChecks)
	}
	if !reflect.DeepEqual(src.SkipMailChecks, []string{"spf"}) {
		t.Errorf("skip_mail_checks = %v, want [spf]", src.SkipMailChecks)
	}
}

func TestFromEnvIgnoresInvalidBooleans(t *testing.T) {
	t.Setenv("SITE_HEALTH_VERBOSE", "not-a-bool")
	t.Setenv("SITE_HEALTH_WHOIS", "not-a-bool")

	src := FromEnv()
	if src.Verbose != nil {
		t.Errorf("verbose = %v, want nil", src.Verbose)
	}
	if src.WhoisOnly != nil {
		t.Errorf("whois = %v, want nil", src.WhoisOnly)
	}
}

func TestFromEnvEmptyFormatIgnored(t *testing.T) {
	t.Setenv("SITE_HEALTH_FORMAT", "   ")

	src := FromEnv()
	if src.Format != nil {
		t.Errorf("format = %v, want nil", src.Format)
	}
}

func TestMergeBool(t *testing.T) {
	trueVal := true
	falseVal := false

	if got := MergeBool(false, true, false, &trueVal); got != false {
		t.Errorf("explicit false override = %v, want false", got)
	}
	if got := MergeBool(false, false, true, &falseVal); got != false {
		t.Errorf("env override = %v, want false", got)
	}
	if got := MergeBool(false, false, true, nil, &trueVal); got != true {
		t.Errorf("file override = %v, want true", got)
	}
	if got := MergeBool(false, false, true); got != true {
		t.Errorf("fallback = %v, want true", got)
	}
}

func TestMergeString(t *testing.T) {
	env := "env"
	file := "file"

	if got := MergeString("cli", true, "fallback", &env, &file); got != "cli" {
		t.Errorf("explicit override = %q, want cli", got)
	}
	if got := MergeString("", false, "fallback", &env, &file); got != "env" {
		t.Errorf("env override = %q, want env", got)
	}
	if got := MergeString("", false, "fallback", nil, &file); got != "file" {
		t.Errorf("file override = %q, want file", got)
	}
	if got := MergeString("", false, "fallback"); got != "fallback" {
		t.Errorf("fallback = %q, want fallback", got)
	}
}

func TestMergeStringSlice(t *testing.T) {
	env := []string{"mx"}
	file := []string{"spf"}

	if got := MergeStringSlice([]string{"cli"}, true, env, file); !reflect.DeepEqual(got, []string{"cli"}) {
		t.Errorf("explicit override = %v, want [cli]", got)
	}
	if got := MergeStringSlice(nil, false, env, file); !reflect.DeepEqual(got, []string{"mx"}) {
		t.Errorf("env override = %v, want [mx]", got)
	}
	if got := MergeStringSlice(nil, false, nil, file); !reflect.DeepEqual(got, []string{"spf"}) {
		t.Errorf("file override = %v, want [spf]", got)
	}
	if got := MergeStringSlice(nil, false); got != nil {
		t.Errorf("fallback = %v, want nil", got)
	}
}
