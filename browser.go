package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pquerna/otp/totp"
)

const (
	BrowserCloseDelay    = 300 * time.Second
	XPathUsername        = `//*[@id="awsui-input-0"]`
	XPathPassword        = `//*[@id="awsui-input-1"]`
	XPathTOTP            = `//*[@id="awsui-input-2"]`
	XPathAllow1          = `//*[@id="cli_verification_btn"]`
	XPathAllow2          = `//*[@data-testid="allow-access-button"]`
	XPathSuccess         = `//*[@data-analytics-alert="success"]`
	CookieAcceptSelector = `[data-id="awsccc-cb-btn-accept"]`
	// CookieBannerTimeout bounds waiting for the consent banner to appear,
	// animate in, and disappear. The banner reliably shows on the authorization
	// page but slides up over ~1s, so a too-short wait races it and leaves the
	// banner covering the Allow button.
	CookieBannerTimeout = 10 * time.Second
	// DumpTimeout bounds each failure-dump capture so debugging a stuck page
	// can never hang as long as the operation that failed.
	DumpTimeout = 10 * time.Second
)

// diagJS asks the page what element is at the center of each known Allow
// button. This mirrors rod's Interactable check (which also uses
// elementFromPoint) so the dump can name exactly what is producing
// CoveredError when a click fails with "context deadline exceeded".
const diagJS = `() => {
  const describe = n => n ? {
    tag: n.tagName,
    id: n.id || null,
    cls: typeof n.className === 'string' ? n.className : null,
    testid: n.dataset ? n.dataset.testid || null : null,
    dataId: n.dataset ? n.dataset.id || null : null,
  } : null;
  const out = [];
  for (const sel of ['#cli_verification_btn', '[data-testid="allow-access-button"]']) {
    const el = document.querySelector(sel);
    if (!el) { out.push({sel, found: false}); continue; }
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    const cx = r.left + r.width / 2, cy = r.top + r.height / 2;
    const top = document.elementFromPoint(cx, cy);
    out.push({
      sel,
      found: true,
      self: describe(el),
      rect: {x: r.left, y: r.top, w: r.width, h: r.height},
      style: {display: cs.display, visibility: cs.visibility, opacity: cs.opacity, pointerEvents: cs.pointerEvents, zIndex: cs.zIndex},
      disabled: el.disabled,
      ariaDisabled: el.getAttribute('aria-disabled'),
      atPoint: describe(top),
      covered: top !== el && !el.contains(top),
    });
  }
  return JSON.stringify(out);
}`

// dismissCookieBanner waits for the AWS cookie consent banner, accepts it, and
// waits for it to disappear so it can't cover the authorization buttons.
//
// It is best-effort: if the banner never appears it logs and returns. The wait
// matters because the banner mounts and slides in a beat after the page loads;
// clicking Allow before it is dismissed leaves the banner covering the button,
// which previously caused the click to spin until the timeout (see debug dumps).
func dismissCookieBanner(page *rod.Page) {
	log.Debug("Waiting for cookie consent banner...")

	// rod's Element retries until the deadline, so this also waits for the
	// banner to mount instead of racing it.
	btn, err := page.Timeout(CookieBannerTimeout).Element(CookieAcceptSelector)
	if err != nil {
		log.Debug("No cookie banner appeared, continuing...", "error", err)
		return
	}

	// Wait for the banner to finish animating in so the click lands on it.
	if _, err := btn.Timeout(CookieBannerTimeout).WaitInteractable(); err != nil {
		log.Debug("Cookie banner button never became interactable", "error", err)
		return
	}

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		log.Debug("Failed to click cookie accept button", "error", err)
		return
	}

	// Wait for the banner (and thus this button) to disappear so it can't cover
	// the Allow button. WaitInvisible also succeeds if the node is removed.
	if err := btn.Timeout(CookieBannerTimeout).WaitInvisible(); err != nil {
		log.Debug("Cookie banner did not disappear after accept", "error", err)
		return
	}

	log.Debug("Cookie banner dismissed")
}

// Helper function to find an element with consistent error handling
func findElement(
	page *rod.Page,
	xpath string,
	description string,
	timeout time.Duration,
) (*rod.Element, error) {
	log.Debug("Looking for element", "description", description, "xpath", xpath)
	element, err := page.Timeout(timeout).ElementX(xpath)
	if err != nil {
		return nil, fmt.Errorf("%s not found with XPath %s: %v", description, xpath, err)
	}
	log.Debug("Found element", "description", description, "xpath", xpath)
	return element, nil
}

// Helper function to fill a field and submit the form
func fillAndSubmitField(
	page *rod.Page,
	xpath string,
	value string,
	description string,
	timeout time.Duration,
) error {
	field, err := findElement(page, xpath, description, timeout)
	if err != nil {
		return err
	}

	log.Debug("Filling field", "description", description)
	if err := field.Input(value); err != nil {
		return fmt.Errorf("failed to input %s: %v", description, err)
	}

	log.Debug("Submitting form", "description", description)
	if err := field.Type(input.Enter); err != nil {
		return fmt.Errorf("failed to submit %s form: %v", description, err)
	}

	return nil
}

// Helper function to click a button with consistent error handling
func clickButton(page *rod.Page, xpath string, description string, timeout time.Duration) error {
	button, err := findElement(page, xpath, description, timeout)
	if err != nil {
		return err
	}

	log.Debug("Clicking button", "description", description)
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click %s: %v", description, err)
	}

	return nil
}

// Helper function to get 2FA code
func get2FACode(config *Config) (string, error) {
	if config.TwoFA != "" {
		log.Debug("Using 2FA code from command line")
		return config.TwoFA, nil
	}

	if config.TOTPSecret != "" {
		log.Debug("Generating 2FA code from TOTP secret...")
		return totp.GenerateCode(config.TOTPSecret, time.Now())
	}

	twoFA, err := promptForInput("Enter 2FA code: ", false)
	if err != nil {
		return "", fmt.Errorf("failed to get 2FA code interactively: %v", err)
	}
	log.Debugf("Using 2FA code from interactive prompt: %s", twoFA)
	return twoFA, nil
}

// Helper function to check for success message
func checkSuccessMessage(page *rod.Page, timeout time.Duration) error {
	log.Debug("Checking for success message...")
	_, err := page.Timeout(timeout).ElementX(XPathSuccess)
	if err != nil {
		return fmt.Errorf("success message not found with XPath %s: %v", XPathSuccess, err)
	}

	// successText, err := successElement.Text()
	// if err != nil {
	// 	return fmt.Errorf("failed to get success message text: %v", err)
	// }

	// log.Debug("Success page found with text", "text", successText)
	log.Debug("Success message found")
	return nil
}

func automateBrowserLogin(deviceURL string, config *Config) error {
	log.Info("Starting browser automation...")

	// Setup launcher
	if config.ShowBrowser {
		log.Info("Browser will be visible")
	} else {
		log.Info("Running browser in headless mode")
	}
	l := launcher.New().Headless(!config.ShowBrowser)

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %v", err)
	}

	// Connect to browser
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser at %s: %v", url, err)
	}
	defer func() {
		if config.ShowBrowser && err != nil {
			log.Warn(
				"Browser will be closed after delay because of error",
				"delaySeconds",
				BrowserCloseDelay,
			)
			time.Sleep(BrowserCloseDelay)
		}
		if closeErr := browser.Close(); closeErr != nil {
			log.Error("Failed to close browser", "error", closeErr)
		}
	}()

	// Open device URL
	log.Info("Opening device URL", "url", deviceURL)
	page, err := browser.Page(proto.TargetCreateTarget{URL: deviceURL})
	if err != nil {
		return fmt.Errorf("failed to open page %s: %v", deviceURL, err)
	}

	// Run the login steps. On any failure, dump the page state to disk so the
	// run can be investigated later, then propagate the error.
	if err = performLoginSteps(page, config); err != nil {
		dumpFailureInfo(page, config, err)
		return err
	}

	log.Info("Browser automation completed!")
	return nil
}

// performLoginSteps drives the credential, authorization, and success-check
// stages of the AWS SSO flow on an already-opened page. Every step is bounded
// by the single --timeout budget.
func performLoginSteps(page *rod.Page, config *Config) error {
	timeout := time.Duration(config.TimeoutSeconds) * time.Second

	// Fill credentials
	log.Info("Filling AWS SSO credentials...")
	if err := fillAndSubmitField(page, XPathUsername, config.Username, "username field", timeout); err != nil {
		return err
	}

	if err := fillAndSubmitField(page, XPathPassword, config.Password, "password field", timeout); err != nil {
		return err
	}

	// Get 2FA code and submit
	twoFA, err := get2FACode(config)
	if err != nil {
		return fmt.Errorf("failed to get 2FA code: %v", err)
	}

	if err := fillAndSubmitField(page, XPathTOTP, twoFA, "2FA field", timeout); err != nil {
		return err
	}

	// Authorize access
	log.Info("Authorizing AWS CLI access...")

	// Dismiss cookie banner if it appears on the authorization page
	dismissCookieBanner(page)

	if err := clickButton(page, XPathAllow1, "first Allow button", timeout); err != nil {
		return err
	}

	if err := clickButton(page, XPathAllow2, "second Allow button", timeout); err != nil {
		return err
	}

	// Verify login success
	if err := checkSuccessMessage(page, timeout); err != nil {
		return err
	}

	return nil
}

// dumpFailureInfo writes the page HTML, a screenshot, and a metadata summary to
// the debug directory so failures can be investigated to improve reliability.
// It is best-effort: each capture is bounded by DumpTimeout and individual
// failures are logged but never abort the dump. Secrets are never written.
func dumpFailureInfo(page *rod.Page, config *Config, automationErr error) {
	dir := config.DebugDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "awsssologin-failures")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Warn("Could not create debug dump directory", "dir", dir, "error", err)
		return
	}

	base := filepath.Join(dir, "failure-"+time.Now().Format("20060102-150405"))

	// Bound every page interaction so dumping a stuck page can't hang.
	p := page.Timeout(DumpTimeout)

	// Page info (URL + title) for the metadata summary.
	url, title := "<unknown>", "<unknown>"
	if info, err := p.Info(); err != nil {
		log.Warn("Could not read page info for debug dump", "error", err)
	} else {
		url, title = info.URL, info.Title
	}

	// Metadata summary — note we deliberately omit password/2FA/TOTP secrets.
	var meta strings.Builder
	fmt.Fprintf(&meta, "timestamp:       %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&meta, "error:           %v\n", automationErr)
	fmt.Fprintf(&meta, "page_url:        %s\n", url)
	fmt.Fprintf(&meta, "page_title:      %s\n", title)
	fmt.Fprintf(&meta, "username:        %s\n", config.Username)
	fmt.Fprintf(&meta, "timeout_s:       %d\n", config.TimeoutSeconds)
	fmt.Fprintf(&meta, "show_browser:    %t\n", config.ShowBrowser)

	// Interactability diagnostic — mirrors rod's Interactable check by calling
	// elementFromPoint at each Allow button's center, so the dump answers
	// "what is rod hitting CoveredError against?" instead of leaving us to guess.
	if diag, err := p.Eval(diagJS); err != nil {
		log.Warn("Could not capture interactability diagnostics", "error", err)
		fmt.Fprintf(&meta, "diagnostics:     <error: %v>\n", err)
	} else {
		fmt.Fprintf(&meta, "diagnostics:     %s\n", diag.Value.Str())
	}
	writeDumpFile(base+".txt", []byte(meta.String()))

	// Full page HTML.
	if html, err := p.HTML(); err != nil {
		log.Warn("Could not capture page HTML for debug dump", "error", err)
	} else {
		writeDumpFile(base+".html", []byte(html))
	}

	// Screenshot (PNG).
	if shot, err := p.Screenshot(true, nil); err != nil {
		log.Warn("Could not capture screenshot for debug dump", "error", err)
	} else {
		writeDumpFile(base+".png", shot)
	}

	log.Warn("Failure debug info written", "dir", dir, "prefix", filepath.Base(base))
}

// writeDumpFile writes a single debug artifact, logging on failure.
func writeDumpFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Warn("Could not write debug dump file", "path", path, "error", err)
		return
	}
	log.Info("Wrote debug dump file", "path", path)
}
