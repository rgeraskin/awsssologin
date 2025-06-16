# AWS SSO Login Automation

Automate AWS SSO login by reading output from `aws sso login --no-browser` and automatically filling in credentials using browser automation with go-rod.

## Features

- ✅ Reads AWS SSO output from stdin via pipe
- ✅ Extracts device URL from AWS CLI output
- ✅ Automates browser login with username, password, and TOTP
- ✅ Supports multiple credential input methods (CLI, environment variables, interactive)
- ✅ TOTP generation from secret key or manual input
- ✅ Headless browser mode by default with option to show browser
- ✅ Comprehensive error handling and detailed logging
- ✅ Forwards AWS CLI output to maintain original functionality

## Installation

```bash
go build -o awsssologin
```

## Usage

### Basic Usage
```bash
aws sso login --sso-session <session-name> --no-browser | ./awsssologin
```

### With Additional AWS CLI Arguments
```bash
aws sso login --sso-session pashapay --region us-east-1 --no-browser | ./awsssologin
```

### With Credential Flags
```bash
aws sso login --sso-session prod --no-browser | ./awsssologin -u myusername -p mypassword --totp-secret ABCD1234...
```

### Command Line Options

| Flag             | Short | Description                                    |
|------------------|-------|------------------------------------------------|
| `--username`     | `-u`  | AWS SSO username                               |
| `--password`     | `-p`  | AWS SSO password                               |
| `--totp-secret`  |       | TOTP secret key for automatic 2FA generation   |
| `--show-browser` |       | Show browser window (runs headless by default) |
| `--help`         | `-h`  | Show help                                      |

### Examples

#### Interactive Mode (prompts for credentials)
```bash
aws sso login --sso-session production --no-browser | ./awsssologin
```

#### Using Command Line Arguments
```bash
aws sso login --sso-session dev --no-browser | ./awsssologin -u myusername -p mypassword --totp-secret ABCD1234...
```

#### With Visible Browser (for debugging)
```bash
aws sso login --sso-session staging --no-browser | ./awsssologin --show-browser
```

## How It Works

1. You run `aws sso login --no-browser` and pipe the output to `awsssologin`
2. The tool reads the AWS CLI output from stdin and extracts the device URL
3. The tool forwards the original AWS CLI output to stdout (so you still see AWS messages)
4. The tool opens the device URL in a browser and automates the login process
5. Once login is complete, the AWS CLI command finishes successfully

## Credential Priority

Credentials are resolved in the following order (highest to lowest priority):

1. **Command line flags** (`-u`, `-p`, `--totp-secret`)
2. **Environment variables**:
   - `AWSSSOLOGIN_USERNAME`
   - `AWSSSOLOGIN_PASSWORD`
   - `AWSSSOLOGIN_TOTP_SECRET`
3. **Interactive prompts** (secure password input)

## TOTP Handling

- If `--totp-secret` is provided (or `AWSSSOLOGIN_TOTP_SECRET` env var), TOTP codes are generated automatically
- If no TOTP secret is provided, you'll be prompted to enter the 6-digit code manually
- TOTP secret should be the base32-encoded secret from your authenticator app

## Environment Variables

```bash
export AWSSSOLOGIN_USERNAME="your-username"
export AWSSSOLOGIN_PASSWORD="your-password"
export AWSSSOLOGIN_TOTP_SECRET="ABCD1234EFGH5678..."  # Optional
```

## Browser Automation

The tool uses specific XPath selectors optimized for AWS SSO pages:
- Username field: `//*[@id="awsui-input-0"]`
- Password field: `//*[@id="awsui-input-1"]`
- TOTP link: `//*[@id="main-container"]/div[2]/div/div/div[2]/div/form/awsui-form/div/div[2]/span/span/div[4]/div[2]/div/div/div/a`
- TOTP input field: `//*[@id="awsui-input-2"]`
- First Allow button: `//*[@id="cli_verification_btn"]/span`
- Second Allow button: `//*[@id=":rl:"]/div[3]/div/div/div[2]/button/span`
- Success message: `//*[@id="alert-:r10:"]/div[1]`

Runs in headless mode by default for automated workflows, but can show the browser with `--show-browser` for debugging.

## Error Handling

- Detailed logging of each step
- Comprehensive error messages
- Automatic cleanup of browser processes
- Forwards AWS CLI output while processing

## Requirements

- AWS CLI installed and configured
- Go 1.24.2 or later
- Internet connection for browser automation

## Troubleshooting

1. **AWS CLI not found**: Ensure AWS CLI is installed and in your PATH
2. **Browser automation fails**: Try running with `--show-browser` to see what's happening
3. **Form fields not found**: The tool tries multiple selectors, but some SSO pages may use custom ones
4. **TOTP issues**: Verify your TOTP secret is correct and properly base32-encoded
5. **No device URL found**: Ensure you're using `--no-browser` flag with `aws sso login`