# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes with full commit lists are also published on the
[Releases page](https://github.com/jmoyonero/godev/releases).

## [Unreleased]

### Added
- Unit tests for `pkg/config` and `pkg/docker`, with golden files for the generated Dockerfile.
- CI runs `golangci-lint` and `govulncheck`, and Build & Test on both Linux and macOS.
- Dependabot for Go modules and GitHub Actions.
- `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue and pull request templates.

### Fixed
- `godev test` no longer shadows the command arguments.

### Security
- Bump `golang.org/x/sys` to v0.44.0 (GO-2026-5024).

## [0.3.0] - 2026-09-19

### Added
- Prebuilt binaries for Linux, macOS and Windows (amd64/arm64) published with GoReleaser on every tag.
- `godev --version` flag.

### Fixed
- `godev version` reported a hardcoded `v0.1.0`; the version, commit and build date are now injected at build time, with a fallback to the Go build info for `go install`.

## [0.2.0] - 2026-09-19

### Added
- `godev test` formats its output with `gotestsum` when installed.
- `godev test --cover` and `--html` to report coverage.
- `godev e2e` brings the infrastructure up before the suite and tears it down afterwards.

### Changed
- Generated images run on distroless instead of Alpine.
- Every project's local stack runs under one generic Compose project.
- The README and every CLI message are now in English.
- Go 1.27.1 is required.

## [0.1.7] - 2026-09-12

### Added
- Prometheus, Grafana and OpenTelemetry Collector in the dynamic Compose stack.
- `godev run` to build and run microservices concurrently.
- Prebuilt Grafana dashboards: HTTP Client Telemetry and Database Connection Pool.

### Fixed
- Grafana stat panels no longer jitter (`last_over_time` instead of extrapolated queries).
- `godev lint` resolves the golangci-lint module path by major version, so v2 installs correctly.

## [0.1.6] - 2026-09-11

### Added
- `exclude_dirs` for tests, in the configuration and as a CLI flag.

## [0.1.5] - 2026-09-11

### Added
- Auto-detection of a local SSH key to download private Go modules during image builds.

### Fixed
- Error handling when downloading modules in the generated Dockerfile.

## [0.1.4] - 2026-09-11

### Added
- Universal generated Dockerfile (no Dockerfile in the service repo) and `godev build-image`.

## [0.1.3] - 2026-09-11

### Added
- Dynamic Docker Compose generation, so services need no `docker-compose.yaml` of their own.

## [0.1.2] - 2026-09-11

### Added
- Auto-detection of Compose and seed files under `test/infra` and `infra`.

## [0.1.1] - 2026-09-10

### Added
- `godev generate` for `go generate` and mockgen.

### Fixed
- `reset-db` brings up every infrastructure service, including mocks.

## [0.1.0] - 2026-09-10

### Added
- First release of the `godev` CLI: lint, security analysis, tests, local infrastructure and Robot Framework E2E orchestration.
- Shared E2E environment variables, `pg_isready` health check and seeds loaded through stdin.

[Unreleased]: https://github.com/jmoyonero/godev/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/jmoyonero/godev/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/jmoyonero/godev/compare/v0.1.7...v0.2.0
[0.1.7]: https://github.com/jmoyonero/godev/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/jmoyonero/godev/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/jmoyonero/godev/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/jmoyonero/godev/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/jmoyonero/godev/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/jmoyonero/godev/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/jmoyonero/godev/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jmoyonero/godev/releases/tag/v0.1.0
