# site-health

A fast CLI for checking website and domain health.

> **Status:** Go rewrite complete (v0.8)

## Features

Zero external dependencies — stdlib only:

- Domain registration expiry
- Registrar lookup
- Optional verbose output for troubleshooting
- DNS resolution (A, AAAA, CNAME)
- TCP connectivity (80/443)
- HTTP/HTTPS availability
- Redirect and canonical URL validation
- SSL certificate validation
- Response time measurement
- HTML content validation
- Common server/PHP/WordPress error detection
- Parked domain detection
- Final summary with failure and warning counts
- MX record lookup, including Null MX recognition
- SPF TXT record validation
- DMARC TXT record validation
- `llms.txt` availability check
- Machine-readable JSON output for automation and monitoring

## Usage

Run the full website/domain health check. By default, output is a concise dashboard:

```bash
site-health example.com
```

Example:

```text
SITE HEALTH
───────────
● example.com
  HTTPS        OK
  DNS          OK
  SSL          81 days
  Redirect     OK
  Response     184 ms
  Mail         OK
Status: HEALTHY
```

When a forwarded or explicit canonical URL matters, the dashboard includes it:

```text
SITE HEALTH
───────────
● example.com
  Expected     https://example.org/
  HTTPS        OK
  DNS          OK
  SSL          81 days
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

Forwarded domains are detected automatically when the final URL is unambiguous:

```bash
site-health example.com
```

To strictly require a specific final URL, provide it explicitly:

```bash
site-health --expected-url https://example.org/ example.com
```

Output a machine-readable JSON document instead of the dashboard (useful for scripts, CI, and monitoring). `--verbose` output is suppressed in JSON mode; the same exit codes apply (`0` healthy, `1` issues found):

```bash
site-health --format json example.com
```

Example:

```json
{
  "tool": "site-health",
  "version": "0.8",
  "domain": "example.com",
  "mode": "site",
  "expected_url": "https://example.com/",
  "forwarding": {
    "auto_detected": false,
    "ambiguous": false,
    "hint_url": null,
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

In mail-only mode, the `checks` object contains just the `mail` block and the top-level `mode` is `"mail"`.

## Installation

### Build from source

Clone the repository and build:

```bash
git clone https://github.com/atillalab/site-health.git
cd site-health
go build -o site-health .
```

The resulting binary has zero external dependencies.

### Install with go install

```bash
go install github.com/atillalab/site-health@latest
```

### Pre-built binary

Download the latest release from [GitHub Releases](https://github.com/atillalab/site-health/releases).

## Development

### Run tests

```bash
go test ./...
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
