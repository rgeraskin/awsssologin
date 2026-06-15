# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A CLI that automates the browser half of `aws sso login --no-browser --use-device-code`. The AWS CLI prints a device URL and then blocks polling `CreateToken`; this tool reads that URL, drives a headless browser (go-rod) through the SSO login + authorization pages, and lets the AWS CLI complete.

## Commands

```bash
go build -o awsssologin .        # build
go vet ./... && gofmt -l .       # vet + format check (gofmt -l lists unformatted files; -w fixes)
go test ./...                    # full tests — launches headless Chromium (downloads on first run)
go test -short ./...             # skip browser-dependent tests (no Chromium needed)
go test -run TestDumpFailureInfo -v   # run a single test
goreleaser build --snapshot --clean --single-target -o /tmp/x   # local release-build sanity check
```

There is no separate linter beyond `go vet` + `gofmt`.

## Architecture

Three files form a linear pipeline, all in `package main`:

- **main.go** — cobra CLI + stdin handling. Finds the device URL via `deviceURLPattern` regex, or takes `--device-url` directly.
- **config.go** — credential resolution with strict precedence: **CLI flag → env var (`AWSSSOLOGIN_*`) → interactive prompt**. Interactive prompts only work with `--device-url` (stdin is consumed by the pipe otherwise).
- **browser.go** — all go-rod automation: fill username/password/2FA, dismiss the cookie banner, click two "Allow" buttons, verify success.
- **version.go** — `version`/`commit`/`date` vars injected by GoReleaser at release; fall back to `debug.ReadBuildInfo()` for `go install`/`go build`.

### Two subtleties that drove most recent work

1. **stdin must be drained only on success.** When run via pipe, the upstream `aws sso login` keeps polling until the device code expires (~10 min). Draining its remaining output on the *failure* path blocks reporting the error for that whole time. So `runSSO` drains stdin (`continueReadingStdin`) only after `automateBrowserLogin` succeeds; on failure it returns immediately, closing the pipe so `aws` also stops. Do not move this into a `defer`.

2. **go-rod timeout & "covered" semantics.** `page.Timeout(d)` sets a context deadline that the returned element inherits, so element-find *and* the subsequent click share one budget. A click on a button that exists but is covered (e.g. by the cookie consent banner) makes `WaitInteractable` retry on `CoveredError` until that deadline — this is why a single `--timeout` governs every step. `dismissCookieBanner` therefore tracks the banner lifecycle (wait for mount → interactable → invisible) rather than polling once and bailing.

### Failure diagnostics

On any automation failure, `dumpFailureInfo` writes `failure-<ts>.{html,png,txt}` to `--debug-dir` (default OS temp). The `.txt` includes a runtime `elementFromPoint` probe (`diagJS`) at each Allow button's center, which names the element causing `CoveredError`. Each capture is bounded by `DumpTimeout` so dumping a stuck page can't itself hang. Secrets are never written to dumps.

## Releasing

Pushing a `v*` tag triggers `.github/workflows/goreleaser.yml`, which builds binaries, creates the GitHub Release, and updates the Homebrew formula in `rgeraskin/homebrew-homebrew`. Nothing ships from branch/PR pushes — the tag is the lever.
