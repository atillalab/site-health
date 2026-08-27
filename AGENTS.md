# AGENTS.md

## Project

`site-health` is a stdlib-only Go CLI for checking website and domain health.

## Commands

- Test: `go test ./...`
- Vet: `go vet ./...`
- Build local binary: `go build -o bin/site-health .`
- Check version: `bin/site-health --version`

## Release Notes

- Version is defined in `internal/version/version.go`.
- README examples include the current version and should be kept in sync.
- Homebrew formula lives in sibling repo `../homebrew-tap/Formula/site-health.rb`.
- For a release:
  - commit changes in this repo
  - tag and push `vX.Y.Z`
  - compute the GitHub source tarball SHA
  - update and push the Homebrew tap formula

## Constraints

- Keep the CLI stdlib-only unless there is a strong reason to add dependencies.
- Do not commit `bin/site-health`; it is ignored local build output.
