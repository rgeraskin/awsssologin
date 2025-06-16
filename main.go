package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/spf13/cobra"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSO(&config)
		},
	}

	rootCmd.Flags().StringVarP(&config.Username, "username", "u", "", "AWS SSO username")
	rootCmd.Flags().StringVarP(&config.Password, "password", "p", "", "AWS SSO password")
	rootCmd.Flags().StringVar(&config.TOTPSecret, "totp-secret", "", "TOTP secret key for 2FA")
	rootCmd.Flags().
		StringVar(&config.DeviceURL, "device-url", "", "AWS SSO device URL (if provided, stdin will be ignored)")
	rootCmd.Flags().
		BoolVar(&config.ShowBrowser, "show-browser", false, "Show browser window (runs headless by default)")

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func runSSO(config *Config) error {
	log.Println("Starting AWS SSO login automation...")

	// Step 1: Get credentials
	if err := getCredentials(config); err != nil {
		return fmt.Errorf("failed to get credentials: %v", err)
	}

	// Step 2: Get device URL either from command line or stdin
	var deviceURL string
	var err error

	if config.DeviceURL != "" {
		deviceURL = config.DeviceURL
		log.Printf("Using device URL from command line: %s", deviceURL)
	} else {
		deviceURL, err = readDeviceURLFromStdin()
		if err != nil {
			return fmt.Errorf("failed to read device URL from stdin: %v", err)
		}
		log.Printf("Device URL found from stdin: %s", deviceURL)
	}

	// Step 3: Automate browser login
	if err := automateBrowserLogin(deviceURL, config); err != nil {
		return fmt.Errorf("browser automation failed: %v", err)
	}

	log.Println("AWS SSO login completed successfully!")
	return nil
}

func readDeviceURLFromStdin() (string, error) {
	log.Println("Reading AWS SSO output from stdin to find device URL...")

	urlRegex := regexp.MustCompile(
		`https://pashapay\.awsapps\.com/start/#/device\?user_code=[A-Z0-9-]+`,
	)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		// log.Printf("AWS output: %s", line)

		// Also forward the line to stdout so the user can see the original AWS output
		// fmt.Println(line)

		if match := urlRegex.FindString(line); match != "" {
			log.Printf("Device URL found: %s", match)
			return match, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading from stdin: %v", err)
	}

	return "", fmt.Errorf("device URL not found in AWS SSO output")
}
