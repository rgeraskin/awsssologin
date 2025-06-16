package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pquerna/otp/totp"
)

const (
	DefaultTimeout = 20 * time.Second
	XPathUsername  = `//*[@id="awsui-input-0"]`
	XPathPassword  = `//*[@id="awsui-input-1"]`
	XPathTOTP      = `//*[@id="awsui-input-2"]`
	XPathTOTPLink  = `//*[@id="main-container"]/div[2]/div/div/div[2]/div/form/awsui-form/div/div[2]/span/span/div[4]/div[2]/div/div/div/a`
	XPathAllow1    = `//*[@id="cli_verification_btn"]/span`
	XPathAllow2    = `//*[@id=":rl:"]/div[3]/div/div/div[2]/button/span`
	XPathSuccess   = `//*[@id="alert-:r10:"]/div[1]`
)

func automateBrowserLogin(deviceURL string, config *Config) error {
	log.Println("Starting browser automation...")

	// Setup launcher
	if config.ShowBrowser {
		log.Println("Browser will be visible")
	} else {
		log.Println("Running browser in headless mode")
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
		if err := browser.Close(); err != nil {
			log.Printf("Warning: failed to close browser: %v", err)
		}
	}()

	// Open device URL
	log.Printf("Opening device URL: %s", deviceURL)
	page, err := browser.Page(proto.TargetCreateTarget{URL: deviceURL})
	if err != nil {
		return fmt.Errorf("failed to open page %s: %v", deviceURL, err)
	}

	// Wait for page to load
	log.Println("Waiting for page to load...")
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for page load: %v", err)
	}

	// Find username field using specific XPath
	log.Println("Looking for username field...")
	usernameField, err := page.Timeout(DefaultTimeout).ElementX(XPathUsername)
	if err != nil {
		return fmt.Errorf("username field not found with XPath %s: %v", XPathUsername, err)
	}
	log.Println("Found username field")

	// Fill username
	log.Println("Filling username...")
	if err := usernameField.Input(config.Username); err != nil {
		return fmt.Errorf("failed to input username: %v", err)
	}

	// Submit the form by pressing Enter key in the password field
	log.Println("Submitting username form...")
	if err := usernameField.Type(input.Enter); err != nil {
		return fmt.Errorf("failed to submit username form: %v", err)
	}

	// Find password field using specific XPath
	log.Println("Looking for password field...")
	passwordField, err := page.Timeout(DefaultTimeout).ElementX(XPathPassword)
	if err != nil {
		return fmt.Errorf("password field not found with XPath %s: %v", XPathPassword, err)
	}
	log.Println("Found password field")

	// Fill password
	log.Println("Filling password...")
	if err := passwordField.Input(config.Password); err != nil {
		return fmt.Errorf("failed to input password: %v", err)
	}

	// Submit the form by pressing Enter key in the password field
	log.Println("Submitting login form...")
	if err := passwordField.Type(input.Enter); err != nil {
		return fmt.Errorf("failed to submit login form: %v", err)
	}

	// Find TOTP field using specific XPath
	log.Println("Looking for TOTP field...")
	totpField, err := page.Timeout(DefaultTimeout).ElementX(XPathTOTP)
	if err != nil {
		return fmt.Errorf("TOTP field not found with XPath %s: %v", XPathTOTP, err)
	}
	log.Println("Found TOTP field")

	// Generate or get TOTP code
	var totpCode string
	if config.InteractiveTOTP {
		totpCode, err = promptForInput("Enter TOTP code: ", false)
		if err != nil {
			return fmt.Errorf("failed to get TOTP code: %v", err)
		}
	} else {
		log.Println("Generating TOTP code from secret...")
		totpCode, err = totp.GenerateCode(config.TOTPSecret, time.Now())
		if err != nil {
			return fmt.Errorf("failed to generate TOTP code: %v", err)
		}
	}

	log.Println("Filling TOTP code...")
	if err := totpField.Input(totpCode); err != nil {
		return fmt.Errorf("failed to input TOTP code: %v", err)
	}

	// Submit TOTP form
	log.Println("Submitting TOTP form...")
	if err := totpField.Type(input.Enter); err != nil {
		return fmt.Errorf("failed to submit TOTP form: %v", err)
	}

	// Look for first Allow button (CLI verification)
	log.Println("Looking for first Allow button...")
	allowButton1, err := page.Timeout(DefaultTimeout).ElementX(XPathAllow1)
	if err == nil {
		log.Println("Found first Allow button, clicking it...")
		if err := allowButton1.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("failed to click first Allow button: %v", err)
		}
		time.Sleep(3 * time.Second)
	} else {
		return fmt.Errorf("first Allow button not found with XPath %s: %v", XPathAllow1, err)
	}

	// Look for second Allow button (final authorization)
	log.Println("Looking for second Allow button...")
	allowButton2, err := page.Timeout(DefaultTimeout).ElementX(XPathAllow2)
	if err == nil {
		log.Println("Found second Allow button, clicking it...")
		if err := allowButton2.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return fmt.Errorf("failed to click second Allow button: %v", err)
		}
		time.Sleep(3 * time.Second)
	} else {
		return fmt.Errorf("second Allow button not found with XPath %s: %v", XPathAllow2, err)
	}

	// Check for success message
	log.Println("Checking for success message...")
	successElement, err := page.Timeout(DefaultTimeout).ElementX(XPathSuccess)
	if err == nil {
		successText, err := successElement.Text()
		if err != nil {
			return fmt.Errorf("failed to get success message text: %v", err)
		}
		log.Printf("Success page found with text: %s", successText)
	} else {
		return fmt.Errorf("success message not found with XPath %s: %v", XPathSuccess, err)
	}

	log.Println("Browser automation completed!")
	return nil
}
