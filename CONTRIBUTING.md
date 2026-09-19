# Contributing to godev

Thanks for your interest in improving `godev`! This guide explains how to set up
the project, the checks every change must pass and how releases are cut.

By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Reporting bugs and requesting features

- Search the [existing issues](https://github.com/jmoyonero/godev/issues) first.
- Open a new issue with the **Bug report** or **Feature request** template.
- Security problems must **not** be reported in public issues: see [SECURITY.md](SECURITY.md).

## Development setup

Requirements:

- Go (the version in [`go.mod`](go.mod))
- Git
- Docker with Compose v2, only to try `infra`, `run`, `e2e` and `build-image` by hand

```bash
git clone https://github.com/jmoyonero/godev.git
cd godev
go build ./...
go run . --help
```

Project layout:

| Path | Contents |
|---|---|
| `main.go` | Entry point |
| `cmd/` | One file per Cobra command (`lint.go`, `test.go`, `infra.go`...) |
| `pkg/config` | `.godev.yaml` loading, defaults and auto-detection |
| `pkg/docker` | Universal Dockerfile generator |
| `pkg/infra` | Dynamic Docker Compose, Grafana dashboards, port handling |
| `pkg/execx` | Process execution helpers |
| `pkg/ui` | Console output |

## Checks

Every pull request runs these checks in CI, and all of them must pass. Run them
locally before pushing:

```bash
go test -race ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go mod tidy && git diff --exit-code go.mod go.sum
```

Formatting is enforced by golangci-lint (`gofmt` and `goimports` with this
module as the local import group). Apply it with:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 fmt ./...
```

### Tests

- Add or update tests for every behavior change. Prefer table-driven tests.
- Code that reads the working directory should run from `t.TempDir()` with `t.Chdir`.
- Never launch real tools from command tests. Every external process goes
  through `pkg/execx`, so install the fake runner with `execxtest.Install(t)`
  and assert on the recorded commands. In `cmd/`, the `execute` and `setup`
  helpers in `helpers_test.go` run a command in a temporary project and reset
  the flags afterwards.
- The generated Dockerfile is checked against golden files in
  `pkg/docker/testdata/`. After an intended change, regenerate and review them:

  ```bash
  go test ./pkg/docker -update
  git diff pkg/docker/testdata
  ```

## Commits and pull requests

- Branch from `main` and open the pull request against `main`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):
  `feat:`, `fix:`, `docs:`, `test:`, `ci:`, `chore:`, `refactor:`, with an optional
  scope such as `feat(infra):`. Release notes are grouped from these prefixes.
- Keep pull requests focused on one change and fill in the template.
- Add user-facing changes to the `Unreleased` section of [CHANGELOG.md](CHANGELOG.md).

## Releases (maintainers)

Versions follow [Semantic Versioning](https://semver.org/). To release:

1. Move the `Unreleased` entries in `CHANGELOG.md` under the new version and merge.
2. Tag `main` and push the tag:

   ```bash
   git tag -a v0.4.0 -m "v0.4.0"
   git push origin v0.4.0
   ```

The `Release` workflow runs [GoReleaser](https://goreleaser.com), which builds the
binaries, injects the version, commit and date, and publishes the GitHub release.
To try it locally without publishing:

```bash
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```
