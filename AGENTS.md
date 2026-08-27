# AGENTS.md

## Project

`site-health` is a stdlib-only Go CLI for checking website and domain health.

## Commands

- Test: `go test ./...`
- Vet: `go vet ./...`
- Build local binary: `go build -o bin/site-health .`
- Check version: `bin/site-health --version`
- Run web UI: `bin/site-health web [--port <port>]`

## Versioning

This project follows [Semantic Versioning](https://semver.org) (`MAJOR.MINOR.PATCH`):

- **MAJOR** — incompatible changes to CLI behavior, flag semantics, or output formats.
- **MINOR** — new backward-compatible features (e.g., a new flag like `--doctor`).
- **PATCH** — backward-compatible bug fixes.

Pre-1.0 versions (`0.x.x`) indicate initial development. Even before `1.0.0`, use minor bumps for features and patch bumps for fixes.

Version is defined in `internal/version/version.go`. To bump it across all related files, run:

```bash
scripts/bump-version.sh <major|minor|patch>
```

This updates:

- `internal/version/version.go`
- `README.md`
- `../homebrew-tap/Formula/site-health.rb` (URL only; SHA must be updated manually after release)

Always review the resulting diff before committing.

## Release Notes

- Version is defined in `internal/version/version.go`.
- README examples include the current version and should be kept in sync.
- Homebrew formula lives in sibling repo `../homebrew-tap/Formula/site-health.rb`.
- Whenever `internal/version/version.go` is updated, update the Homebrew tap formula for the same version.
- For a release:
  - commit changes in this repo
  - tag and push `vX.Y.Z`
  - compute the GitHub source tarball SHA
  - update the Homebrew tap formula with the matching tag and SHA
  - commit and push the Homebrew tap formula

## Constraints

- Keep the CLI stdlib-only unless there is a strong reason to add dependencies.
- Do not commit `bin/site-health`; it is ignored local build output.
