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
	// DexURLRegex matches a Dex OIDC auth-code URL on a stdin line. It is keyed
	// on the redirect_uri query parameter (always present in the auth-code flow,
	// and what the dex flow waits on) rather than a fixed host, so it works for
	// any Dex instance — argocd or otherwise.
	DexURLRegex    = `https?://[^\s'"]+[?&]redirect_uri=[^\s'"]+`
	DefaultTimeout = 30
	// StdinURLSource is the flag value that tells a URL flag to read its URL
	// from stdin instead of taking the value literally.
	StdinURLSource = "-"
)

var (
	// Pre-compiled regexes for better performance
	deviceURLPattern           = regexp.MustCompile(DeviceURLRegex)
	deviceURLValidationPattern = regexp.MustCompile("^" + DeviceURLRegex + "$")
	dexURLPattern              = regexp.MustCompile(DexURLRegex)
)

func main() {
	var config Config

	rootCmd := &cobra.Command{
		Use:     "awsssologin",
		Version: versionString(),
		Short:   "Automate AWS SSO login with browser automation",
		Long: `Automate AWS SSO login by reading output from 'aws sso login --no-browser'
and automatically filling in credentials using browser automation.

Two flows are supported. The URL flow and source are always explicit: pass a
literal URL, or '-' to read that flow's URL from stdin.
  • AWS device-code: --device-url <url>, or --device-url - to read from a pipe.
  • Dex OIDC auth-code: --dex-url <url>, or --dex-url - to read from a pipe.
    Useful for tools that log in through Dex backed by AWS IAM Identity Center,
    e.g. 'argocd login --grpc-web <server> --sso --sso-launch-browser=false'.

Usage:
  aws sso login --sso-session <session> --no-browser | awsssologin --device-url -
  argocd login --grpc-web <server> --sso --sso-launch-browser=false 2>&1 | awsssologin --dex-url -
  awsssologin --dex-url '<dex auth URL printed by the CLI>'

Credentials can be provided via:
1. Command line flags (highest priority)
2. Environment variables (AWSSSOLOGIN_USERNAME, AWSSSOLOGIN_PASSWORD, AWSSSOLOGIN_2FA, AWSSSOLOGIN_TOTP_SECRET)
3. Interactive prompts (lowest priority). Works only with --device-url flag!`,
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

	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	rootCmd.Flags().StringVarP(&config.Username, "username", "u", "", "AWS SSO username")
	rootCmd.Flags().StringVarP(&config.Password, "password", "p", "", "AWS SSO password")
	rootCmd.Flags().StringVarP(&config.TwoFA, "2fa", "", "", "AWS SSO 2FA code")
	rootCmd.Flags().
		StringVarP(&config.TOTPSecret, "totp-secret", "t", "", "TOTP secret key for 2FA (if not provided, you'll be prompted to enter TOTP interactively)")
	rootCmd.Flags().
		StringVar(&config.DeviceURL, "device-url", "", "AWS SSO device URL, or '-' to read it from stdin (e.g. piped from 'aws sso login --no-browser')")
	rootCmd.Flags().
		StringVar(&config.DexURL, "dex-url", "", "Dex OIDC auth URL for the auth-code flow (e.g. 'argocd login --sso --sso-launch-browser=false'), or '-' to read it from stdin; mutually exclusive with --device-url")
	rootCmd.Flags().
		BoolVar(&config.ShowBrowser, "show-browser", false, "Show browser window (runs headless by default)")
	rootCmd.Flags().
		IntVar(&config.TimeoutSeconds, "timeout", DefaultTimeout, "Timeout in seconds for browser operations")
	rootCmd.Flags().
		StringVar(&config.DebugDir, "debug-dir", "", "Directory to write failure debug dumps (HTML, screenshot, info); defaults to the OS temp dir")
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

	// Step 2: Get the login URL. The flow (dex vs device) and the source (stdin
	// vs literal) are both declared explicitly by the flags: a value of "-" means
	// "read this flow's URL from stdin". Only a stdin path keeps a scanner so the
	// upstream CLI's output can be drained on success. When the URL is read from
	// stdin its flow flag is rewritten to the resolved URL so downstream steps
	// (e.g. dexCallbackPrefix) see the real value.
	switch {
	case config.DexURL == StdinURLSource:
		deviceURL, scanner, err = readURLFromStdin(dexURLPattern, "Dex")
		if err != nil {
			return fmt.Errorf("failed to process stdin: %v", err)
		}
		config.DexURL = deviceURL
	case config.DexURL != "":
		deviceURL = config.DexURL
		log.Info("Using Dex auth URL from command line", "url", deviceURL)
	case config.DeviceURL == StdinURLSource:
		deviceURL, scanner, err = readURLFromStdin(deviceURLPattern, "device")
		if err != nil {
			return fmt.Errorf("failed to process stdin: %v", err)
		}
	case config.DeviceURL != "":
		deviceURL = config.DeviceURL
		log.Info("Using device URL from command line", "url", deviceURL)
	}

	// Step 3: Automate browser login
	if err := automateBrowserLogin(deviceURL, config); err != nil {
		// Fail fast: do NOT drain stdin here. The upstream "aws sso login
		// --use-device-code" keeps polling CreateToken until the device code
		// expires (~10 min), so draining would block reporting this error for
		// that whole time. Exiting now closes the pipe and lets aws stop too.
		return fmt.Errorf("browser automation failed: %v", err)
	}

	// On success, drain the remaining AWS CLI output so it can finish writing
	// the token without a broken pipe.
	if scanner != nil {
		continueReadingStdin(scanner)
	}

	log.Info("AWS SSO login completed successfully!")
	return nil
}

func continueReadingStdin(scanner *bufio.Scanner) {
	log.Debug("Reading remaining AWS CLI output...")
	for scanner.Scan() {
		line := scanner.Text()
		// log.Debug("AWS output: %s", line)
		fmt.Println(line) // Forward to user
	}

	if err := scanner.Err(); err != nil {
		log.Errorf("failed to read remaining AWS CLI output: %v", err)
	}
}

// readURLFromStdin scans stdin line by line for the first URL matching pattern
// and returns it along with the still-open scanner so the caller can drain the
// rest of the upstream CLI's output on success. kind is used only for logging
// and error messages ("device", "Dex").
func readURLFromStdin(pattern *regexp.Regexp, kind string) (string, *bufio.Scanner, error) {
	log.Info("Reading CLI output from stdin to find URL...", "kind", kind)

	scanner := bufio.NewScanner(os.Stdin)

	var found string
	for scanner.Scan() {
		line := scanner.Text()

		if match := pattern.FindString(line); match != "" {
			log.Info("URL found from stdin", "kind", kind, "url", match)
			found = match
			break // Stop reading; the upstream CLI is now blocked waiting on us.
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("error reading from stdin: %v", err)
	}

	if found == "" {
		return "", nil, fmt.Errorf("%s URL not found in stdin output", kind)
	}

	return found, scanner, nil
}
