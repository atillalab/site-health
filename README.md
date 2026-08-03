# site-health

A fast CLI for checking website and domain health.

> **Status:** 🚧 Early development (Bash prototype)

## Features

Current checks include:

- Domain registration expiry
- DNS resolution (A, AAAA, CNAME)
- TCP connectivity (80/443)
- HTTP/HTTPS availability
- Redirect and canonical URL validation
- SSL certificate validation
- Response time measurement
- HTML content validation
- Common server/PHP/WordPress error detection
- Parked domain detection
- MX record lookup
- `llms.txt` availability check

## Usage

```bash
site-health example.com
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