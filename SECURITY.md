# Security Policy

## Supported versions

Only the latest release of `godev` receives security fixes. Please upgrade
before reporting an issue.

| Version | Supported |
|---|---|
| Latest release | ✅ |
| Older releases | ❌ |

## Reporting a vulnerability

**Do not open a public issue for security problems.**

Report them privately through GitHub:
[Security → Report a vulnerability](https://github.com/jmoyonero/godev/security/advisories/new).

Please include:

- The affected version (`godev version`) and your OS.
- A description of the issue and its impact.
- Steps or a minimal project to reproduce it.

You can expect an acknowledgement within 7 days. Once the issue is confirmed, a
fix is released as soon as possible and credited to you in the release notes,
unless you prefer to remain anonymous.

## Scope

`godev` runs developer tools and containers on your machine using the
configuration in your project's `.godev.yaml`. Running `godev` on an untrusted
repository is equivalent to running its build scripts: only use it on projects
you trust.

### Build credentials

`godev build-image` only handles an SSH key when the project declares private
modules (`docker.private_modules`, or the owner of the module in `go.mod`), and
only when you point at one: `--ssh-key`, `docker.ssh_key` in `.godev.yaml`,
`SSH_DEPLOY_KEY_B64` or `SSH_DEPLOY_KEY`. Keys are never read from `~/.ssh`
implicitly.

The key is written to a private temporary file and mounted as a BuildKit secret
on the `go mod download` layer alone. It is not passed as a build argument, so
it stays out of the image, out of the build metadata and out of the machine's
process list, and the temporary file is removed when the build ends. Prefer a
repository deploy key with read-only access over a personal key.

Dependencies are scanned with `govulncheck` in CI on every change and weekly.
