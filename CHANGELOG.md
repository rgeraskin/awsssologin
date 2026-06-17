# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Dex OIDC auth-code login flow via `--dex-url`, for CLIs that log in through Dex
  backed by AWS IAM Identity Center (e.g. `argocd login --sso --sso-launch-browser=false`).
  The tool reads the printed Dex auth URL, reuses the AWS sign-in form, and completes
  when the browser is redirected to the CLI's local callback (`redirect_uri`).
- `-` as an explicit stdin source for `--device-url` and `--dex-url`
  (e.g. `aws sso login --no-browser | awsssologin --device-url -`).

### Changed

- The URL source is now always explicit. Bare invocation with no URL flag is an
  error; piped device-code logins must pass `--device-url -`.
- Interactive credential prompts now work with a literal `--dex-url <url>` as well
  as `--device-url <url>`; they remain disabled when the URL is read from stdin (`-`).

### Documentation

- Document the `--version` flag.

## [0.3.0] - 2026-06-15

### Added

- `--version` flag.
- Failure diagnostics: on any browser-automation failure, dump the page HTML, a
  screenshot, and a metadata summary to `--debug-dir` (default OS temp), including a
  covered-element probe that names what is covering the Allow button. Secrets are
  never written.

### Changed

- Fail fast on automation failure: return immediately and close the pipe instead of
  waiting for the device code to expire (~10 min).

### Fixed

- Reliably dismiss the AWS cookie consent banner before authorization by tracking its
  full lifecycle (mount → interactable → invisible).

### Documentation

- Add `CLAUDE.md`.

## [0.2.1] - 2026-01-14

### Fixed

- Dismiss the AWS cookie consent banner before clicking the authorization buttons.

### Documentation

- README: more details about using password manager CLIs.

## [0.2.0] - 2025-08-21

### Added

- `--2fa` flag to pass a 2FA code directly on the command line.

## [0.1.0] - 2025-06-25

### Added

- Initial release: automate the browser half of
  `aws sso login --no-browser --use-device-code` by driving a headless browser
  (go-rod) through the SSO login and authorization pages.
- Credential resolution with strict precedence: CLI flag → environment variable
  (`AWSSSOLOGIN_*`) → interactive prompt.
- TOTP code generation from a secret (`-t`/`--totp-secret`), or a manually entered
  2FA code.
- Direct device URL input via `--device-url` (bypassing the AWS CLI pipe).
- Basic configuration validation and optimized URL-pattern validation.
- Configurable timeout and log levels; enhanced logging.
- Homebrew distribution via GoReleaser.

### Fixed

- Broken pipe error when forwarding AWS CLI output.

[Unreleased]: https://github.com/rgeraskin/awsssologin/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/rgeraskin/awsssologin/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/rgeraskin/awsssologin/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/rgeraskin/awsssologin/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/rgeraskin/awsssologin/releases/tag/v0.1.0
