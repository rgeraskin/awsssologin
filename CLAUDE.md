# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A CLI that automates the browser half of a blocked SSO login. Two cases:

1. `aws sso login --no-browser --use-device-code` — the AWS CLI prints a device URL and blocks polling `CreateToken`; this tool reads that URL, drives a headless browser (go-rod) through the SSO login + authorization pages, and lets the AWS CLI complete.
2. A Dex OIDC auth-code login such as `argocd login --grpc-web <server> --sso --sso-launch-browser=false` — the CLI starts a local callback server (e.g. `http://localhost:8085/auth/callback`), prints a Dex auth URL, and blocks. Because that Dex instance federates to AWS IAM Identity Center, the browser lands on the **same** AWS sign-in form; this tool fills it and waits for the browser to be redirected back to the local callback, which unblocks the CLI.

The URL flow and source are **always explicit** — there is no implicit stdin default. Pass a literal URL (`--device-url <url>` / `--dex-url <url>`) or `-` to read that flow's URL from stdin (`--device-url -` / `--dex-url -`); the flags are mutually exclusive, and `"-"` is the `StdinURLSource` sentinel. `aws … | awsssologin --device-url -`, `argocd … 2>&1 | awsssologin --dex-url -`.

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

- **main.go** — cobra CLI + URL routing. The flow (device vs dex) and source (literal vs stdin) are always explicit via the flags: `--device-url`/`--dex-url` take a literal URL, or `-` to read it from stdin (matched by `deviceURLPattern`/`dexURLPattern`). There is no implicit default; bare invocation is an error.
- **config.go** — credential resolution with strict precedence: **CLI flag → env var (`AWSSSOLOGIN_*`) → interactive prompt**. Interactive prompts work only when a literal URL is given (`--device-url <url>`/`--dex-url <url>`); they're disabled under stdin (`-`), gated by `usesStdin()`, because the pipe owns stdin.
- **browser.go** — all go-rod automation. Username+password entry is shared; then it branches by flow. **Device flow** (`performDeviceAuthSteps`): mandatory 2FA, dismiss cookie banner, click two "Allow" buttons, verify on-page success element. **Dex flow** (`performDexAuthSteps`, selected by `--dex-url`): no Allow buttons; 2FA is *conditional* — `waitForMFAOrCallback` polls for whichever comes first, the MFA field or the redirect; success is the browser URL reaching the `redirect_uri` origin (the CLI's localhost callback server), not a DOM element. **Caveat:** the no-MFA branch is unverified in practice — go-rod launches an ephemeral browser profile each run, so AWS never sees a remembered device and always challenges MFA. The branch is defensive/correct-by-construction but dormant; only the MFA-required path has been exercised live.
- **version.go** — `version`/`commit`/`date` vars injected by GoReleaser at release; fall back to `debug.ReadBuildInfo()` for `go install`/`go build`.

### Two subtleties that drove most recent work

1. **stdin must be drained only on success.** When run via pipe, the upstream `aws sso login` keeps polling until the device code expires (~10 min). Draining its remaining output on the *failure* path blocks reporting the error for that whole time. So `runSSO` drains stdin (`continueReadingStdin`) only after `automateBrowserLogin` succeeds; on failure it returns immediately, closing the pipe so `aws` also stops. Do not move this into a `defer`.

2. **go-rod timeout & "covered" semantics.** `page.Timeout(d)` sets a context deadline that the returned element inherits, so element-find *and* the subsequent click share one budget. A click on a button that exists but is covered (e.g. by the cookie consent banner) makes `WaitInteractable` retry on `CoveredError` until that deadline — this is why a single `--timeout` governs every step. `dismissCookieBanner` therefore tracks the banner lifecycle (wait for mount → interactable → invisible) rather than polling once and bailing.

### Failure diagnostics

On any automation failure, `dumpFailureInfo` writes `failure-<ts>.{html,png,txt}` to `--debug-dir` (default OS temp). The `.txt` includes a runtime `elementFromPoint` probe (`diagJS`) at each Allow button's center, which names the element causing `CoveredError`. Each capture is bounded by `DumpTimeout` so dumping a stuck page can't itself hang. Secrets are never written to dumps.

## Releasing

Pushing a `v*` tag triggers `.github/workflows/goreleaser.yml`, which builds binaries, creates the GitHub Release, and updates the Homebrew formula in `rgeraskin/homebrew-homebrew`. Nothing ships from branch/PR pushes — the tag is the lever.
