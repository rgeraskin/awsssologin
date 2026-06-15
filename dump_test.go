package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	// Page with a deliberate transparent full-viewport overlay covering the
	// first Allow button — mirrors the exact failure mode we want diagnostics
	// to detect (rod's elementFromPoint hitting the overlay, not the button).
	page, err := browser.Page(proto.TargetCreateTarget{
		URL: "data:text/html," +
			"<html><head><title>Smoke</title></head><body>" +
			"<button id='cli_verification_btn' style='position:relative;z-index:1'>Allow</button>" +
			"<div id='covering-overlay' data-id='evil' " +
			"style='position:fixed;inset:0;background:transparent;z-index:9999'></div>" +
			"</body></html>",
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

	// The .txt diagnostics field should name the covering overlay for the
	// first Allow button — proves the runtime probe correctly identifies what
	// would produce CoveredError.
	txtMatches, _ := filepath.Glob(filepath.Join(dir, "failure-*.txt"))
	txt, err := os.ReadFile(txtMatches[0])
	if err != nil {
		t.Fatalf("read txt: %v", err)
	}
	got := string(txt)
	t.Logf("dump txt:\n%s", got)
	if !strings.Contains(got, `"covered":true`) {
		t.Errorf("expected diagnostics to mark first Allow button as covered, got: %s", got)
	}
	if !strings.Contains(got, `"id":"covering-overlay"`) {
		t.Errorf("expected diagnostics to name the covering overlay element, got: %s", got)
	}
}
