package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"

	"github.com/charmbracelet/log"

	"github.com/spf13/cobra"
)

const (
	DeviceURLRegex = `https://[a-zA-Z0-9-]+\.awsapps\.com/start/#/device\?user_code=[A-Z0-9-]+`
	DefaultTimeout = 30
)

var (
	// Pre-compiled regexes for better performance
	deviceURLPattern           = regexp.MustCompile(DeviceURLRegex)
	deviceURLValidationPattern = regexp.MustCompile("^" + DeviceURLRegex + "$")
)

func main() {
	var config Config

	rootCmd := &cobra.Command{
		Use:   "awsssologin",
		Short: "Automate AWS SSO login with browser automation",
		Long: `Automate AWS SSO login by reading output from 'aws sso login --no-browser'
and automatically filling in credentials using browser automation.

Usage:
  aws sso login --sso-session <session> --no-browser | awsssologin

Credentials can be provided via:
1. Command line flags (highest priority)
2. Environment variables (AWSSSOLOGIN_USERNAME, AWSSSOLOGIN_PASSWORD, AWSSSOLOGIN_TOTP_SECRET)
3. Interactive prompts (lowest priority)`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			logLevel, err := log.ParseLevel(config.LogLevel)
			if err != nil {
				return fmt.Errorf("invalid log level: %v", err)
			}
			log.SetLevel(logLevel)
			return runSSO(&config)
		},
	}

	rootCmd.Flags().StringVarP(&config.Username, "username", "u", "", "AWS SSO username")
	rootCmd.Flags().StringVarP(&config.Password, "password", "p", "", "AWS SSO password")
	rootCmd.Flags().
		StringVarP(&config.TOTPSecret, "totp-secret", "t", "", "TOTP secret key for 2FA (if not provided, you'll be prompted to enter TOTP interactively)")
	rootCmd.Flags().
		StringVar(&config.DeviceURL, "device-url", "", "AWS SSO device URL (if provided, stdin will be ignored)")
	rootCmd.Flags().
		BoolVar(&config.ShowBrowser, "show-browser", false, "Show browser window (runs headless by default)")
	rootCmd.Flags().
		IntVar(&config.TimeoutSeconds, "timeout", DefaultTimeout, "Timeout in seconds for browser operations")
	rootCmd.Flags().
		StringVar(&config.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func runSSO(config *Config) error {
	log.Info("Starting AWS SSO login automation...")

	// Step 0: Validate configuration and set defaults
	if err := config.ValidateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	var (
		deviceURL string
		scanner   *bufio.Scanner
		err       error
	)

	// Step 1: Get credentials
	if err := getCredentials(config); err != nil {
		return fmt.Errorf("failed to get credentials: %v", err)
	}

	// Step 2: Get device URL either from command line or stdin
	if config.DeviceURL != "" {
		deviceURL = config.DeviceURL
		log.Info("Using device URL from command line", "url", deviceURL)
	} else {
		deviceURL, scanner, err = readDeviceURLFromStdin()
		if err != nil {
			return fmt.Errorf("failed to process stdin: %v", err)
		}
	}

	// Step 3: Automate browser login
	if err := automateBrowserLogin(deviceURL, config); err != nil {
		return fmt.Errorf("browser automation failed: %v", err)
	}

	// Step 4: Continue reading remaining AWS CLI output to prevent broken pipe
	if config.DeviceURL == "" {
		log.Debug("Browser automation complete, reading remaining AWS CLI output...")
		err := continueReadingStdin(scanner)
		if err != nil {
			return fmt.Errorf("failed to read remaining AWS CLI output: %v", err)
		}
	}

	log.Info("AWS SSO login completed successfully!")
	return nil
}

func continueReadingStdin(scanner *bufio.Scanner) error {
	for scanner.Scan() {
		line := scanner.Text()
		// log.Debug("AWS output: %s", line)
		fmt.Println(line) // Forward to user
	}

	return scanner.Err()
}

func readDeviceURLFromStdin() (string, *bufio.Scanner, error) {
	log.Info("Reading AWS SSO output from stdin to find device URL...")

	scanner := bufio.NewScanner(os.Stdin)

	// Find device URL
	var deviceURL string
	for scanner.Scan() {
		line := scanner.Text()
		// log.Debug("AWS output: %s", line)
		// fmt.Println(line) // Forward to user

		if match := deviceURLPattern.FindString(line); match != "" {
			log.Info("Device URL found from stdin", "url", match)
			deviceURL = match
			break // Stop reading, AWS CLI is now waiting for our browser automation
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("error reading from stdin: %v", err)
	}

	if deviceURL == "" {
		return "", nil, fmt.Errorf("device URL not found in AWS SSO output")
	}

	return deviceURL, scanner, nil
}
