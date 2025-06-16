package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	"golang.org/x/term"
)

type Config struct {
	Username        string
	Password        string
	TOTPSecret      string
	DeviceURL       string
	ShowBrowser     bool
	TimeoutSeconds  int
	InteractiveTOTP bool
	LogLevel        string
}

// ValidateConfig validates configuration values and sets reasonable defaults
func (c *Config) ValidateConfig() error {
	// Set default timeout if not provided or invalid
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be at least 1 second, got: %d", c.TimeoutSeconds)
	}

	// Validate device URL format if provided
	if c.DeviceURL != "" {
		if err := validateDeviceURL(c.DeviceURL); err != nil {
			return fmt.Errorf("invalid device URL: %v", err)
		}
	}

	return nil
}

// validateDeviceURL checks if the device URL matches the expected AWS SSO pattern
func validateDeviceURL(url string) error {
	if !deviceURLValidationPattern.MatchString(url) {
		return fmt.Errorf("URL does not match expected AWS SSO device URL pattern")
	}
	return nil
}

func getCredentials(config *Config, logLevel log.Level) error {
	log.SetLevel(logLevel)

	// Username cascade: CLI -> ENV -> Interactive
	if config.Username == "" {
		if env := os.Getenv("AWSSSOLOGIN_USERNAME"); env != "" {
			config.Username = env
			log.Info("Using username from environment variable", "username", config.Username)
		} else {
			username, err := promptForInput("Enter AWS SSO username: ", false)
			if err != nil {
				return err
			}
			config.Username = username
		}
	} else {
		log.Info("Using username from command line", "username", config.Username)
	}

	// Password cascade: CLI -> ENV -> Interactive
	if config.Password == "" {
		if env := os.Getenv("AWSSSOLOGIN_PASSWORD"); env != "" {
			config.Password = env
			log.Info("Using password from environment variable")
		} else {
			password, err := promptForInput("Enter AWS SSO password: ", true)
			if err != nil {
				return err
			}
			config.Password = password
		}
	} else {
		log.Info("Using password from command line")
	}

	// TOTP Secret cascade: CLI -> ENV -> Interactive mode
	if config.TOTPSecret == "" {
		if env := os.Getenv("AWSSSOLOGIN_TOTP_SECRET"); env != "" {
			config.TOTPSecret = env
			log.Info("Using TOTP secret from environment variable")
		} else {
			log.Info("No TOTP secret provided - will prompt for TOTP code interactively")
			config.InteractiveTOTP = true
		}
	} else {
		log.Info("Using TOTP secret from command line")
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
