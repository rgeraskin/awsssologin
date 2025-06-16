package main

import (
	"fmt"

	"github.com/charmbracelet/log"

	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pquerna/otp/totp"
)

const (
	BrowserCloseDelay = 300 * time.Second
	XPathUsername     = `//*[@id="awsui-input-0"]`
	XPathPassword     = `//*[@id="awsui-input-1"]`
	XPathTOTP         = `//*[@id="awsui-input-2"]`
	XPathAllow1       = `//*[@id="cli_verification_btn"]`
	XPathAllow2       = `//*[@data-testid="allow-access-button"]`
	XPathSuccess      = `//*[@data-analytics-alert="success"]`
)

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

	// time.Sleep(3 * time.Second)
	return nil
}

// Helper function to get TOTP code based on configuration
func getTOTPCode(config *Config) (string, error) {
	if config.InteractiveTOTP {
		return promptForInput("Enter TOTP code: ", false)
	}

	log.Debug("Generating TOTP code from secret...")
	return totp.GenerateCode(config.TOTPSecret, time.Now())
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
	timeout := time.Duration(config.TimeoutSeconds) * time.Second

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

	// // Wait for page to load
	// log.Debug("Waiting for page to load...")
	// err = page.Timeout(timeout).WaitLoad()
	// if err != nil {
	// 	return fmt.Errorf("failed to wait for page load: %v", err)
	// }

	// Fill credentials
	log.Info("Filling AWS SSO credentials...")
	err = fillAndSubmitField(page, XPathUsername, config.Username, "username field", timeout)
	if err != nil {
		return err
	}

	err = fillAndSubmitField(page, XPathPassword, config.Password, "password field", timeout)
	if err != nil {
		return err
	}

	// Get TOTP code and submit
	totpCode, err := getTOTPCode(config)
	if err != nil {
		return fmt.Errorf("failed to get TOTP code: %v", err)
	}

	err = fillAndSubmitField(page, XPathTOTP, totpCode, "TOTP field", timeout)
	if err != nil {
		return err
	}

	// Authorize access
	log.Info("Authorizing AWS CLI access...")
	err = clickButton(page, XPathAllow1, "first Allow button", timeout)
	if err != nil {
		return err
	}

	err = clickButton(page, XPathAllow2, "second Allow button", timeout)
	if err != nil {
		return err
	}

	// Verify login success
	err = checkSuccessMessage(page, timeout)
	if err != nil {
		return err
	}

	log.Info("Browser automation completed!")
	return nil
}
