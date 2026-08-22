# TO DO Next: `www.example.com` Input Handling

## Problem

Passing a domain that already starts with `www.` causes the tool to treat `www.example.com` as if it were the apex domain. This produces a cascade of false failures because the checks are designed for apex/root domains.

### Reproduction

```bash
./bin/site-health www.example.com
```

Observed failures:

```
FAIL  MX www.example.com — no MX record found
FAIL  SPF www.example.com — no SPF TXT record found
FAIL  DNS www.www.example.com — no DNS record found
FAIL  DMARC _dmarc.www.example.com — no DMARC TXT record found
FAIL  canonical page redirected to unexpected URL: https://example.com/
FAIL  http://www.example.com — redirected to https://example.com/
FAIL  http://www.www.example.com — DNS resolution failed
FAIL  https://www.www.example.com — DNS resolution failed
FAIL  https://www.example.com — redirected to https://example.com/
FAIL  domain registration expiry date could not be read
```

## Root Cause Analysis

1. **Input normalization**: `domain.Normalize()` preserves the `www.` prefix. It strips schemes, paths, ports, and trailing dots, but does not canonicalize `www.example.com` to `example.com`.
2. **Synthetic `www.` lookup**: `CheckDNS()` appends `www.` to the input unconditionally, resulting in the nonsensical host `www.www.example.com`.
3. **HTTP probes**: `CheckHTTP()` probes `http://www.example.com`, `http://www.www.example.com`, `https://www.www.example.com`, and `https://www.example.com`. The `www.www` variants fail to resolve.
4. **Mail checks**: MX/SPF/DMARC are queried against `www.example.com`, which typically has no mail records.
5. **WHOIS**: `CheckDomainRegistration()` queries WHOIS for `www.example.com`, which is not a registered domain.
6. **Canonical redirect mismatch**: The expected URL is `https://www.example.com/`, but many sites redirect `www.example.com` to the apex `https://example.com/`, causing a FAIL.

## Possible Solutions (to be evaluated)

### Option A: Strip `www.` during normalization

Always treat `www.example.com` as `example.com` at the input stage.

- **Pros**: Simple, matches common user intent, eliminates all false failures.
- **Cons**: Users cannot run checks specifically against the `www` hostname; if a site intentionally serves different content on `www`, that distinction is lost.

### Option B: Detect `www.` prefix and behave like an apex alias

If input is `www.example.com`:

- Strip `www.` for DNS/WHOIS/mail checks.
- Use `www.example.com` only for HTTP/SSL checks.
- Set the expected URL to `https://example.com/` if `www.example.com` redirects there, or keep `https://www.example.com/` if it serves directly.

- **Pros**: Preserves ability to check the `www` host while avoiding apex-only check failures.
- **Cons**: More complex; need to track both the original host and the apex domain.

### Option C: Add a `--www` / `--strip-www` flag

Let the user decide whether to treat `www.example.com` as the apex domain.

- **Pros**: Explicit control.
- **Cons**: Adds CLI complexity; most users probably expect `www.example.com` to just work.

### Option D: Improve forwarding detection

Keep input as `www.example.com`, but enhance forwarding detection so that a redirect to `example.com` is accepted as the canonical URL, similar to how apex-to-www redirects are already handled.

- **Pros**: Minimal change, builds on existing forwarding logic.
- **Cons**: Does not solve `www.www` DNS lookups, mail checks, or WHOIS failures.

## Recommended Next Step

Decide on the intended semantics:

- Should `site-health www.example.com` be equivalent to `site-health example.com`?
- Or should it specifically diagnose the `www` hostname while still using `example.com` for apex-only checks (DNS apex, WHOIS, mail)?

Once the behavior is decided, update:

- `internal/domain/domain.go` normalization logic (if Option A or B).
- `internal/check/dns.go` to avoid `www.www` lookups.
- `internal/check/http.go` probe list generation.
- `internal/check/whois.go` to use the apex domain for registration lookup.
- Mail checks to target the apex domain or be skipped when the input is a `www` alias.
- Unit tests in `internal/domain/domain_test.go` and affected check tests.

## Notes

- This is closely related to the recent subdomain support work. The same `IsSubdomain()` / `ApexDomain()` helpers may be reusable here.
- A `www.` prefix is technically a subdomain, but in practice it is almost always a well-known alias of the apex domain. Treating it specially is reasonable.
