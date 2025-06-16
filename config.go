package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Config struct {
	Username        string
	Password        string
	TOTPSecret      string
	DeviceURL       string
	ShowBrowser     bool
	InteractiveTOTP bool
}

func getCredentials(config *Config) error {
	// Username cascade: CLI -> ENV -> Interactive
	if config.Username == "" {
		if env := os.Getenv("AWSSSOLOGIN_USERNAME"); env != "" {
			config.Username = env
			fmt.Println("Using username from environment variable")
		} else {
			username, err := promptForInput("Enter AWS SSO username: ", false)
			if err != nil {
				return err
			}
			config.Username = username
		}
	} else {
		fmt.Println("Using username from command line")
	}

	// Password cascade: CLI -> ENV -> Interactive
	if config.Password == "" {
		if env := os.Getenv("AWSSSOLOGIN_PASSWORD"); env != "" {
			config.Password = env
			fmt.Println("Using password from environment variable")
		} else {
			password, err := promptForInput("Enter AWS SSO password: ", true)
			if err != nil {
				return err
			}
			config.Password = password
		}
	} else {
		fmt.Println("Using password from command line")
	}

	// TOTP Secret cascade: CLI -> ENV -> Interactive mode
	if config.TOTPSecret == "" {
		if env := os.Getenv("AWSSSOLOGIN_TOTP_SECRET"); env != "" {
			config.TOTPSecret = env
			fmt.Println("Using TOTP secret from environment variable")
		} else {
			fmt.Println("No TOTP secret provided - will prompt for TOTP code interactively")
			config.InteractiveTOTP = true
		}
	} else {
		fmt.Println("Using TOTP secret from command line")
	}

	return nil
}

func promptForInput(prompt string, secure bool) (string, error) {
	fmt.Print(prompt)

	if secure {
		bytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Add newline after password input
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}
