# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes with full commit lists are also published on the
[Releases page](https://github.com/jmoyonero/godev/releases).

## [Unreleased]

### Changed
- **Breaking.** The generated Dockerfile no longer hardcodes the maintainer's GitHub organization. `GOPRIVATE` and the SSH rewrite use the prefix in `docker.private_modules`, or the owner of the module declared in `go.mod` when it is not set. A project whose dependencies are all public gets a builder stage with no `GOPRIVATE`, no `openssh-client` and no `SSH_DEPLOY_KEY_B64`, and `build-image` no longer hands a key to Docker there.
- **Breaking.** The local stack no longer defaults to the `loaney_db` database owned by `admin`: `db_name` defaults to `<name>_db` taken from `.godev.yaml` (`app_db` when the project has no name) and `db_user` to `postgres`. Projects relying on the old defaults must set `infra.db_name` and `infra.db_user` explicitly.
- The database container runs in `UTC` instead of `Europe/Madrid`.
- **Breaking.** `build-image` hands the SSH deploy key to Docker as a BuildKit secret mounted on the `go mod download` layer, instead of the `SSH_DEPLOY_KEY_B64` build argument, so it no longer appears in the build metadata or in the machine's process list. Building a project with private modules now requires BuildKit, the default since Docker 23.
- **Breaking.** The deploy key is never taken from `~/.ssh` implicitly. It comes from `--ssh-key`, from the new `docker.ssh_key` option or from `SSH_DEPLOY_KEY_B64` / `SSH_DEPLOY_KEY`, and an unreadable path is now an error instead of a silent fallback.
- `go.mod` declares `go 1.27` instead of `go 1.27.1`, so Go 1.27.0 no longer has to download a newer toolchain to install godev.

### Added
- `docker.ssh_key` in `.godev.yaml`: the path of the key that downloads the project's private modules, with `~` expanded.

## [0.5.1] - 2026-09-19

### Fixed
- `godev infra up`, `run` and `e2e` only wait for the infrastructure services they start. A project without WireMock no longer waits 5s for it on every start, and one without a database no longer retries `pg_isready` for 15s. `godev infra up db` waits only for the database.
- The summary printed after `infra up` lists only the services that were started.

## [0.5.0] - 2026-09-19

### Fixed
- `godev run --reset-db` and `godev e2e` with seeds no longer tear down and restart the stack they had just brought up.
- `godev run` exits with an error when a service crashes, instead of stopping everything with status 0.
- Service binaries are built into a private temporary directory instead of fixed `/tmp/godev-*` paths, which did not exist on Windows.

### Changed
- `godev run` starts the services in the declared order and waits for each `health_url` before starting the next, like `godev e2e`, so a service can rely on those declared before it.
- `godev run` checks the requested services exist before touching the infrastructure.

### Project
- Tests for `run` and `e2e`; `pkg/execx` gains `Start` for background processes, with a controllable fake in `execxtest`.

## [0.4.0] - 2026-09-19

### Fixed
- `godev test --race=false` and `test.race: false` now disable the race detector; before, the flag's default always turned it back on.
- `test.shuffle` from `.godev.yaml` is honored; before, the `--shuffle` default always overrode it.
- `godev infra reset-db` checks the seeds file exists before tearing down and restarting the stack.

### Changed
- A failing command (linter findings, red tests, a missing file...) prints only the error; the usage help is shown for usage mistakes such as an unknown flag.
- `godev generate` reports mockgen failures as `<error>: <stderr>`.

### Security
- Bump `golang.org/x/sys` to v0.44.0 (GO-2026-5024).

### Project
- Unit tests for `pkg/config`, `pkg/docker`, `pkg/execx` and every command except `run` and `e2e`, backed by a fake process runner (`pkg/execx/execxtest`).
- CI runs `golangci-lint` and `govulncheck`, and Build & Test on both Linux and macOS.
- Dependabot for Go modules and GitHub Actions.
- `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue and pull request templates.

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

[Unreleased]: https://github.com/jmoyonero/godev/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/jmoyonero/godev/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/jmoyonero/godev/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/jmoyonero/godev/compare/v0.3.0...v0.4.0
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
