# site-health

A fast CLI for checking website and domain health.

> **Status:** 🚧 Early development (Bash prototype)

## Features

Current checks include:

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

## Usage

Run the full website/domain health check. By default, output is a concise dashboard:

```bash
site-health example.com
```

Example:

```text
SITE HEALTH
────────────────────────────────────────
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
────────────────────────────────────────
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
────────────────────────────────────────
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

## Installation

Clone the repository:

```bash
git clone https://github.com/atillalab/site-health.git
```

Create a symlink in a directory included in your `PATH`:

```bash
mkdir -p ~/.local/bin
ln -s "$(pwd)/site-health/bin/site-health" ~/.local/bin/site-health
```

Add `~/.local/bin` to your `PATH` if necessary:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Verify the installation:

```bash
site-health example.com
```

To update the tool later:

```bash
cd site-health
git pull
```

## Roadmap

- Rewrite as a standalone Go CLI
- JSON output
- CSV output
- Parallel scanning
- Configuration file support
- HTML reports

## License

MIT
