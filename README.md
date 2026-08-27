# site-health

A fast CLI for checking website and domain health.

> **Status:** Go rewrite complete (v0.12.0)

## Quick Start

```bash
site-health example.com
site-health --verbose example.com
site-health --mail example.com
site-health --skip-mail example.com
site-health --mail-checks spf example.com
site-health --skip-redirect example.com
site-health --format json example.com
site-health --doctor
site-health --init-config
```

## Features

Zero external dependencies — stdlib only:

- Domain registration expiry
- Registrar lookup
- Optional verbose output for troubleshooting
- DNS resolution (A, AAAA, CNAME)
- TCP connectivity (80/443)
- HTTP/HTTPS availability
- Redirect and canonical host validation
- SSL certificate validation
- Response time measurement
- HTML content validation
- Common server/PHP/WordPress error detection
- Parked domain detection
- Final summary with failure and warning counts
- MX record lookup, including Null MX recognition
- SPF TXT record validation
- DMARC TXT record validation
- Mail checks skippable in site mode with `--skip-mail`
- Individual mail checks selectable with `--mail-checks` and `--skip-mail-checks`
- Optional `/llms.txt` availability check (skippable with `--skip-llms-txt`)
- Canonical redirect check skippable with `--skip-redirect`
- Machine-readable JSON output for automation and monitoring
- Config file and environment variables for default flags
- Self-diagnostic `--doctor` mode for the binary and local environment

## Usage

Run the full website/domain health check. By default, output is a concise dashboard:

```bash
site-health example.com
```

Example:

```text
Site Health Check
Domain: example.com
Expected Host: example.com

SITE HEALTH
───────────
● example.com
  DNS          OK
  HTTPS        OK
  SSL          81 days
  Domain Reg   757 days (14 Sep 2028)
  Redirect     OK
  Response     184 ms
  Mail         OK
Status: HEALTHY
```

When a forwarded or explicit canonical URL matters, the dashboard includes it:

```text
Site Health Check
Domain: example.com
Expected Host: example.org

SITE HEALTH
───────────
● example.com
  DNS          OK
  HTTPS        OK
  SSL          81 days
  Domain Reg   757 days (14 Sep 2028)
  Redirect     OK
  Response     184 ms
  Mail         OK
Status: HEALTHY
```

Show detailed troubleshooting diagnostics instead of the dashboard:

```bash
site-health --verbose example.com
```

Run only mail-related DNS checks:

```bash
site-health --mail example.com
```

Mail mode checks only:

- MX
- SPF
- DMARC

Example:

```text
MAIL HEALTH
───────────
● example.com
  MX           OK
  SPF          OK
  DMARC        WARN
Status: WARNING
```

Show detailed mail diagnostics:

```bash
site-health --mail --verbose example.com
```

Run only selected mail checks:

```bash
site-health --mail-checks spf example.com
site-health --mail-checks mx,dmarc example.com
site-health --mail --mail-checks spf example.com
```

Skip selected mail checks:

```bash
site-health --skip-mail-checks spf example.com
site-health --mail --skip-mail-checks spf example.com
```

The supported mail check names are `mx`, `spf`, and `dmarc`. `--mail-checks` and `--skip-mail-checks` filter the mail portion of the current run. Use them with `--mail` for mail-only output, or without `--mail` for a full site health run with filtered mail checks. Use `--skip-mail` when you want a full site health run without mail checks.

Skip mail-related DNS checks in a full site health run:

```bash
site-health --skip-mail example.com
```

This skips MX, SPF, and DMARC checks when mail health is outside the monitoring scope. For domains you control that deliberately do not send or receive mail, explicit no-mail DNS policy is still preferred: Null MX, SPF `-all`, and DMARC `p=reject`.

Skip the optional `/llms.txt` check:

```bash
site-health --skip-llms-txt example.com
```

Skip the canonical redirect check when the site intentionally serves the same content from multiple URL variants (for example, both `https://example.com` and `https://www.example.com`):

```bash
site-health --skip-redirect example.com
```

This keeps HTTPS, response-time, and SSL checks running, but does not enforce a single canonical URL. Use it for domains where redirecting every variant to one URL is not desired.

Forwarded domains are detected automatically when the final URL is unambiguous:

```bash
site-health example.com
```

To strictly require a specific final host, provide it explicitly:

```bash
site-health --expected-host example.org example.com
```

Output a machine-readable JSON document instead of the dashboard (useful for scripts, CI, and monitoring). `--verbose` output is suppressed in JSON mode; the same exit codes apply (`0` healthy, `1` issues found):

```bash
site-health --format json example.com
```

Example:

```json
{
  "tool": "site-health",
  "version": "0.12.0",
  "domain": "example.com",
  "mode": "site",
  "expected_host": "example.com",
  "forwarding": {
    "auto_detected": false,
    "ambiguous": false,
    "candidates": []
  },
  "checks": {
    "dns": { "status": "OK", "a": ["104.20.23.154"], "aaaa": [] },
    "https": { "status": "OK" },
    "ssl": { "status": "OK", "days_remaining": 81, "subject": "CN=example.com" },
    "redirect": { "status": "OK" },
    "response": { "status": "OK", "ms": 184 },
    "domain_registration": {
      "status": "OK",
      "registrar": "MarkMonitor Inc.",
      "expires_at": "2028-09-14T04:00:00Z",
      "days_remaining": 757
    },
    "mail": {
      "status": "OK",
      "mx": { "status": "OK", "records": ["0 ."] },
      "spf": { "status": "OK", "records": ["v=spf1 -all"] },
      "dmarc": { "status": "OK", "records": ["v=DMARC1;p=reject"] }
    }
  },
  "issues": [],
  "summary": { "failures": 0, "warnings": 0, "status": "HEALTHY" }
}
```

In mail-only mode, the `checks` object contains just the `mail` block and the top-level `mode` is `"mail"`. When mail checks are skipped with `--skip-mail`, the `mail` block is omitted. When individual mail checks are skipped with `--mail-checks` or `--skip-mail-checks`, only the enabled mail subchecks appear under `checks.mail`.

Run self-diagnostics for the binary and local environment. `--doctor` does not require a domain and checks the install path, detected installation method, current and latest versions, system time, config file, DNS resolution, outbound HTTPS, WHOIS connectivity, and mail DNS capabilities:

```bash
site-health --doctor
```

Example:

```text
site-health doctor
──────────────────
Binary path:      /opt/homebrew/bin/site-health
Install method:   Homebrew
Current version:  0.12.0
Latest version:   0.12.0    (up to date)

Environment
───────────
System time:        OK  2026-08-27T12:34:56Z
Config file:        OK  not configured
                    /Users/alice/.config/site-health/config.json
DNS resolution:     OK  example.com → 93.184.216.34
Outbound HTTPS:     OK  https://detectportal.firefox.com/success.txt → 200 (success)
WHOIS lookup:       OK  whois.iana.org:43 reachable
Mail DNS (MX):      OK  1 record(s)
Mail DNS (SPF):     OK  record found
Mail DNS (DMARC):   OK  record found
Status: HEALTHY
```

The latest-version check uses the GitHub Releases API. If the network is unavailable or the API rate-limit is exceeded, the check reports the error gracefully without failing the whole command.

## Configuration

Set default behavior in a config file so you do not have to repeat flags for every run. The default config path follows the XDG Base Directory Specification:

```bash
$XDG_CONFIG_HOME/site-health/config.json
```

If `XDG_CONFIG_HOME` is unset, it falls back to:

```bash
~/.config/site-health/config.json
```

Use `--config <path>` to load a different file. A missing config file is silently ignored; a malformed file exits with an error.

Generate a starter config file with all supported settings:

```bash
site-health --init-config
```

This writes the sample file to the default config path. Use `--config <path>` to write it elsewhere. It will not overwrite an existing file.

Example config:

```json
{
  "verbose": false,
  "skip_redirect": true,
  "skip_mail": false,
  "skip_llms_txt": false,
  "format": "json",
  "mail_checks": ["mx", "spf"],
  "skip_mail_checks": []
}
```

Supported settings:

| Setting            | Type       | Matching flag          |
|--------------------|------------|------------------------|
| `verbose`          | boolean    | `--verbose`            |
| `skip_redirect`    | boolean    | `--skip-redirect`      |
| `skip_mail`        | boolean    | `--skip-mail`          |
| `skip_llms_txt`    | boolean    | `--skip-llms-txt`      |
| `format`           | string     | `--format`             |
| `mail_checks`      | string[]   | `--mail-checks`        |
| `skip_mail_checks` | string[]   | `--skip-mail-checks`   |

The same options can be set via environment variables:

```bash
export SITE_HEALTH_FORMAT=json
export SITE_HEALTH_SKIP_REDIRECT=true
export SITE_HEALTH_MAIL_CHECKS=mx,spf
```

Precedence, from highest to lowest:

1. Explicit CLI flags
2. Environment variables (`SITE_HEALTH_*`)
3. Config file values
4. Built-in defaults

When `--verbose` is enabled and a config file is loaded, the path is printed to stderr so it is clear where defaults are coming from.

## Installation

### Homebrew

Preferred installation method on macOS:

```bash
brew install atillalab/tap/site-health
```

### Install with go install

```bash
go install github.com/atillalab/site-health@latest
```

### Pre-built binary

Download the latest release for your platform from [GitHub Releases](https://github.com/atillalab/site-health/releases).

### Build from source

Clone the repository and build:

```bash
git clone https://github.com/atillalab/site-health.git
cd site-health
go build -o site-health .
```

The resulting binary has zero external dependencies.

## Exit codes

- `0` — healthy, no failures
- `1` — one or more checks failed
- `2` — usage error (missing domain, invalid flag, etc.)

## Development

### Run tests

```bash
go test ./...
```

Run tests with readable, per-test output:

```bash
go test ./... -v
```

### Build

```bash
go build -o site-health .
```

### Vet

```bash
go vet ./...
```

## License

MIT
