# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Cast is a Go CLI that wraps SaltStack to install "Cast-compatible" distributions (primarily SIFT and REMnux, with a v2 format for general distros). It is the successor to `sift-cli`. `install` only runs on Linux; `release` and `test-state` are developer-facing and cross-platform.

## Common commands

- Build: `go build -o bin/cast main.go` (the `Dockerfile` does the same).
- Tests: `go test -timeout 60s ./...` (matches CI in `.github/workflows/tests.yml`).
- Run a single test: `go test ./pkg/<pkg> -run TestName -v`.
- Lint: `golangci-lint run` (config in `.golangci.yml`; enabled linter set is hand-picked, not `enable-all`).
- Snapshot release locally: `goreleaser release --clean --snapshot` (uses `.goreleaser.yml`, writes to `release/`). Tag-based real releases happen in GitHub Actions.
- Version metadata is injected at link time via `-X github.com/ekristen/cast/pkg/common.{SUMMARY,BRANCH,VERSION,COMMIT}=...` — running from `go build` without those flags yields `1.0.0-dev` / `dev` / `dirty` as defined in `pkg/common/version.go`.

## Architecture

### Command registration pattern
`main.go` uses blank imports (`_ "github.com/ekristen/cast/pkg/commands/install"`, etc.) so each command package's `init()` runs and calls `common.RegisterCommand`, appending to a package-level slice returned by `common.GetCommands()`. **When adding a new subcommand: create a package under `pkg/commands/<name>/`, register it in its `init()`, and add a blank import to `main.go`.** The CLI framework is `urfave/cli/v3`. `pkg/commands/global.go` defines flags (`--log-level`, etc.) and a `Before` hook that every command appends/uses.

### The `install` pipeline (`pkg/commands/install/install.go`)
1. Resolve the argument: if the path exists on disk it's a local distro, otherwise it's parsed as `owner/repo[@version]` or an alias (`sift`, `remnux`). Aliases are hardcoded in `pkg/distro/aliases.go` and are intentionally limited to backwards-compat for SIFT/REMnux — new aliases should not be added.
2. `distro.NewGitHub` / `distro.NewLocal` both return the `distro.Distro` interface defined in `pkg/distro/distro.go`. For GitHub distros this downloads the release archive, verifies cosign (and optionally legacy PGP) signatures, and parses the `manifest.yaml`. v1 releases (SIFT/REMnux) embed hardcoded manifests from `pkg/distro/aliases.go`; v2 releases ship their own `manifest.yaml`.
3. Pillar templates with a `_template` suffix (e.g. `sift_user_template`) are rendered through Go `html/template` + sprig in `Manifest.Render` — the resulting key drops the suffix (`sift_user`). `--variable KEY=VAL` CLI flags feed the template context along with `User` (from `--user` / `$SUDO_USER`).
4. `pkg/installer` then invokes `salt-call` locally. Currently only `--saltstack-install-mode=package` is supported — the code rejects other modes. Output is streamed, regex-parsed to surface state start/completion, and results YAML is written to the cache.
5. Successful installs persist `{distro, version, mode}` to `~/.config/cast/state.yaml` via `pkg/state`, so re-running `cast install` without `--mode` reuses the previous selection.

### The `release` pipeline (`pkg/release/release.go`)
Run from within a Cast distro repo (not this repo). Requires a pre-existing git tag and a `.cast.yml`. It creates a tarball of the repo at the tag, writes a manifest, computes sha256/sha512 checksums, signs with cosign (and legacy PGP via `--legacy-pgp-sign`), and uploads to GitHub Releases. `pkg/commands/init` generates a starter `.cast.yml`.

### The `test-state` command
Spins up a Docker container (default image `ghcr.io/ekristen/cast-tools/saltstack-tester:24.04-3006`), bind-mounts the distro source into `/srv/salt/<name>`, and runs `salt-call state.sls <state>`. Used by distro authors to iterate on states without installing to a host.

### Key directories
- `pkg/common/` — version vars, command registry, embedded cosign/PGP public keys used to verify SIFT/REMnux legacy releases.
- `pkg/distro/` — distro interface + GitHub/local impls + manifest parsing + hardcoded aliases.
- `pkg/saltstack/` — downloads/installs the Salt binary (or uses packages), parses Salt result YAML.
- `pkg/installer/` — orchestrates the `salt-call` invocation.
- `pkg/cosign/`, `pkg/utils/gpg.go` — signature verification.
- `pkg/cache/`, `pkg/state/` — on-disk persistence (`/var/cache/cast`, `~/.config/cast/state.yaml`).

## Cast distro concepts (relevant when changing distro/release logic)

- A **v1 distro** is SIFT/REMnux-style: states live under a subdirectory matching `base_dir`, releases use PGP signing, manifests are not shipped with the release.
- A **v2 distro** places states at the repo root (to enable `git submodule` inclusion), ships a `manifest.yaml` asset, and uses cosign signatures. `cast release` produces v2.
- **Modes** are named aliases for SLS targets (e.g. mode `server` → state `sift.server`); a manifest declares one default. Deprecated modes carry a `replacement` field.
- See `docs/distro.md` and `docs/migrate.md` for the authoring spec — keep these in sync if you change manifest fields.
