package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	"golang.org/x/term"
)

type Config struct {
	Username       string
	Password       string
	TwoFA          string
	TOTPSecret     string
	DeviceURL      string
	DexURL         string
	ShowBrowser    bool
	TimeoutSeconds int
	DebugDir       string
	LogLevel       string
}

// ValidateConfig validates configuration values and sets reasonable defaults
func (c *Config) ValidateConfig() error {
	// Set default timeout if not provided or invalid
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be at least 1 second, got: %d", c.TimeoutSeconds)
	}

	// The two flows are driven by different entry URLs and cannot be combined.
	if c.DeviceURL != "" && c.DexURL != "" {
		return fmt.Errorf("--device-url and --dex-url are mutually exclusive")
	}

	// A URL source is mandatory and always explicit: a literal URL, or "-" to
	// read it from stdin. There is no implicit default.
	if c.DeviceURL == "" && c.DexURL == "" {
		return fmt.Errorf("no URL source: pass --device-url, --dex-url, or either with '-' to read from stdin")
	}

	// Validate device URL format when a literal URL is given ("-" means stdin).
	if c.DeviceURL != "" && c.DeviceURL != StdinURLSource {
		if err := validateDeviceURL(c.DeviceURL); err != nil {
			return fmt.Errorf("invalid device URL: %v", err)
		}
	}

	// Validate Dex URL format when a literal URL is given ("-" means stdin).
	if c.DexURL != "" && c.DexURL != StdinURLSource {
		if err := validateDexURL(c.DexURL); err != nil {
			return fmt.Errorf("invalid dex URL: %v", err)
		}
	}

	return nil
}

// usesStdin reports whether the login URL will be read from stdin, which is the
// case when either URL flag is set to "-". Interactive credential prompts are
// impossible in that case because the pipe owns stdin.
func (c *Config) usesStdin() bool {
	return c.DeviceURL == StdinURLSource || c.DexURL == StdinURLSource
}

// hasIncompleteCredentials returns true if any required credentials are missing
func (c *Config) hasIncompleteCredentials() bool {
	return c.Username == "" || c.Password == "" || (c.TwoFA == "" && c.TOTPSecret == "")
}

// validateDeviceURL checks if the device URL matches the expected AWS SSO pattern
func validateDeviceURL(rawURL string) error {
	if !deviceURLValidationPattern.MatchString(rawURL) {
		return fmt.Errorf("URL does not match expected AWS SSO device URL pattern")
	}
	return nil
}

// validateDexURL checks that the Dex auth URL is a parseable http(s) URL that
// carries a redirect_uri. The redirect_uri is where the browser lands once
// login succeeds, so the dex flow uses it as its success signal — without it
// there is nothing to wait for.
func validateDexURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL is not parseable: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Query().Get("redirect_uri") == "" {
		return fmt.Errorf("URL is missing the redirect_uri query parameter")
	}
	return nil
}

func getCredentials(config *Config) error {
	// Username: CLI -> ENV
	if config.Username == "" {
		if env := os.Getenv("AWSSSOLOGIN_USERNAME"); env != "" {
			config.Username = env
			log.Info("Using username from environment variable", "username", config.Username)
		}
	} else {
		log.Info("Using username from command line", "username", config.Username)
	}

	// Password: CLI -> ENV
	if config.Password == "" {
		if env := os.Getenv("AWSSSOLOGIN_PASSWORD"); env != "" {
			config.Password = env
			log.Info("Using password from environment variable")
		}
	} else {
		log.Info("Using password from command line")
	}

	// 2FA: CLI -> ENV
	if config.TwoFA == "" {
		if env := os.Getenv("AWSSSOLOGIN_2FA"); env != "" {
			config.TwoFA = env
			log.Info("Using 2FA code from environment variable")
		}
	} else {
		log.Info("Using 2FA code from command line")
	}

	// TOTP Secret: CLI -> ENV
	if config.TOTPSecret == "" {
		if env := os.Getenv("AWSSSOLOGIN_TOTP_SECRET"); env != "" {
			config.TOTPSecret = env
			log.Info("Using TOTP secret from environment variable")
		}
	} else {
		log.Info("Using TOTP secret from command line")
	}

	// Interactive prompt doesn't work when the URL comes from stdin, because the
	// pipe owns stdin. It is fine when a literal URL is passed to a flag.
	if config.usesStdin() && config.hasIncompleteCredentials() {
		return fmt.Errorf("interactive prompts don't work when reading the URL from stdin ('-'); provide credentials via flags or environment variables")
	}

	// Interactive prompts
	if config.Username == "" {
		username, err := promptForInput("Enter AWS SSO username: ", false)
		if err != nil {
			return err
		}
		config.Username = username
	}

	if config.Password == "" {
		password, err := promptForInput("Enter AWS SSO password: ", true)
		if err != nil {
			return err
		}
		config.Password = password
	}

	// If no 2FA code or TOTP secret provided, prompt for 2FA code later
	// because we are limited in time for 2FA code
	if config.TwoFA == "" && config.TOTPSecret == "" {
		log.Info("No 2FA code or TOTP secret provided, will prompt for 2FA code later")
	}

	return nil
}

func promptForInput(prompt string, secure bool) (string, error) {
	fmt.Print(prompt)

	var input string
	var err error

	if secure { // password input
		bytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Add newline after password input
		if err != nil {
			return "", fmt.Errorf("failed to read secure input: %v", err)
		}
		input = string(bytes)
	} else { // plain text input
		reader := bufio.NewReader(os.Stdin)
		input, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read plain text input: %v", err)
		}
		input = strings.TrimSpace(input)
	}

	// if input is empty, return error
	if input == "" {
		return "", fmt.Errorf("input is empty")
	}

	return input, nil
}
