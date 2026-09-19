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

Dependencies are scanned with `govulncheck` in CI on every change and weekly.
