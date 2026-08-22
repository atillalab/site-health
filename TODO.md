# Subdomain Support

## Status

Implemented.

## Problem

`site-health` was originally built around apex (root) domains. When a subdomain such as `sub.example.com` was passed as the target, several checks failed because they assumed an apex-domain model.

### Reproduction (before fix)

```bash
./bin/site-health sub.example.com
```

Observed failures:

```
FAIL  MX sub.example.com — no MX record found
FAIL  SPF sub.example.com — no SPF TXT record found
FAIL  DNS www.sub.example.com — no DNS record found
FAIL  DMARC _dmarc.sub.example.com — no DMARC TXT record found
FAIL  domain registration expiry date could not be read
FAIL  http://www.sub.example.com — DNS resolution failed
FAIL  https://www.sub.example.com — DNS resolution failed
```

## Solution

Added subdomain detection and adjusted checks accordingly.

- Added `domain.IsSubdomain()` and `domain.ApexDomain()` helpers in `internal/domain/domain.go`.
- `CheckDNS` skips the synthetic `www.` lookup for subdomains.
- `CheckDomainRegistration` resolves the apex domain before querying WHOIS.
- `CheckHTTP` and `detectForwarding` probe only `http://` and `https://` variants for subdomains (no `www.`).
- In site mode, mail checks (MX/SPF/DMARC) are skipped for subdomains.
- In `--mail` mode, mail checks are run against the apex domain so inherited policy is still validated.

## Verification

```bash
./bin/site-health sub.example.com
# Status: HEALTHY

./bin/site-health --mail sub.example.com
# MX/SPF/DMARC checked against example.com; Status: HEALTHY

./bin/site-health example.com
# Apex domain behavior unchanged; Status: HEALTHY
```

## Known Limitations

`ApexDomain()` uses a simple last-two-label heuristic. It works for common TLDs (`.com`, `.org`, `.net`, `.io`, etc.) but is not accurate for registered domains under multi-label public suffixes such as `.co.uk`. A public-suffix list could be added later if that becomes necessary.
