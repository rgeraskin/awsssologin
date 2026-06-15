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
	BrowserCloseDelay   = 300 * time.Second
	XPathUsername       = `//*[@id="awsui-input-0"]`
	XPathPassword       = `//*[@id="awsui-input-1"]`
	XPathTOTP           = `//*[@id="awsui-input-2"]`
	XPathAllow1         = `//*[@id="cli_verification_btn"]`
	XPathAllow2         = `//*[@data-testid="allow-access-button"]`
	XPathSuccess        = `//*[@data-analytics-alert="success"]`
	XPathCookieAccept   = `//*[@data-id="awsccc-cb-btn-accept" or @data-id="awsccc-cb-btn-decline" or (self::button and normalize-space()="Accept")]`
	CookieBannerTimeout = 2 * time.Second
	// DumpTimeout bounds each failure-dump capture so debugging a stuck page
	// can never hang as long as the operation that failed.
	DumpTimeout = 10 * time.Second
)

// dismissCookieBanner dismisses the AWS cookie consent banner if present.
// Uses a short timeout since the banner may not always appear.
func dismissCookieBanner(page *rod.Page) {
	log.Debug("Checking for cookie consent banner...")

	// Try CSS selectors using rod's native Element method (more reliable than JS eval)
	cssSelectors := []string{
		`[data-id="awsccc-cb-btn-accept"]`,  // AWS cookie consent Accept button
		`[data-id="awsccc-cb-btn-decline"]`, // AWS cookie consent Decline button
	}

	for _, selector := range cssSelectors {
		log.Debug("Trying CSS selector", "selector", selector)
		button, err := page.Timeout(CookieBannerTimeout).Element(selector)
		if err != nil {
			log.Debug("Selector not found", "selector", selector)
			continue
		}

		log.Debug("Cookie banner button found", "selector", selector)

		// Wait for it to be visible
		if err := button.WaitVisible(); err != nil {
			log.Debug("Button not visible", "error", err)
			continue
		}

		// Click it
		if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
			log.Debug("Failed to click cookie button", "error", err)
			continue
		}

		log.Debug("Cookie banner dismissed")
		time.Sleep(500 * time.Millisecond)
		return
	}

	log.Debug("No cookie banner found, continuing...")
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
