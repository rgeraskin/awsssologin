package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Build metadata. These defaults are overridden at release time by GoReleaser
// via -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
// For `go install`/`go build` (no ldflags) they are filled from the embedded
// build info instead, so the version is never hardcoded in more than one place.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// versionString returns a human-readable version line, preferring values
// injected at release time and falling back to the Go build info.
func versionString() string {
	v, c, d := version, commit, date

	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
			case "vcs.time":
				if d == "" {
					d = s.Value
				}
			}
		}
	}

	out := v
	if c != "" {
		if len(c) > 12 {
			c = c[:12]
		}
		out += fmt.Sprintf(" (commit %s)", c)
	}
	if d != "" {
		out += fmt.Sprintf(" built %s", d)
	}
	out += fmt.Sprintf(" %s/%s", runtime.GOOS, runtime.GOARCH)
	return out
}
