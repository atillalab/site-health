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
- SPF TXT record validation
- DMARC TXT record validation
- `llms.txt` availability check

## Usage

```bash
site-health example.com
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
