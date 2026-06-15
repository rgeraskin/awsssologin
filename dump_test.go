package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TestDumpFailureInfo is a smoke test: it loads a real headless page that has
// no Allow button and confirms dumpFailureInfo writes the HTML, screenshot, and
// metadata artifacts to the debug dir.
func TestDumpFailureInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser-dependent smoke test in -short mode")
	}

	dir := t.TempDir()

	controlURL, err := launcher.New().Headless(true).Launch()
	if err != nil {
		t.Fatalf("launch browser: %v", err)
	}

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{
		URL: "data:text/html,<html><head><title>Smoke</title></head><body><h1>no allow button here</h1></body></html>",
	})
	if err != nil {
		t.Fatalf("open page: %v", err)
	}

	config := &Config{Username: "smoke@example.com", TimeoutSeconds: 30, DebugDir: dir}
	dumpFailureInfo(page, config, errors.New("simulated: failed to click first Allow button"))

	for _, ext := range []string{".html", ".png", ".txt"} {
		matches, _ := filepath.Glob(filepath.Join(dir, "failure-*"+ext))
		if len(matches) == 0 {
			t.Fatalf("no %s dump file written to %s", ext, dir)
		}
		info, err := os.Stat(matches[0])
		if err != nil {
			t.Fatalf("stat %s: %v", matches[0], err)
		}
		if info.Size() == 0 {
			t.Fatalf("dump file %s is empty", matches[0])
		}
		t.Logf("wrote %s (%d bytes)", filepath.Base(matches[0]), info.Size())
	}
}
