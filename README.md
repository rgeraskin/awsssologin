# AWS SSO Login Headless Automation

Automate AWS SSO login by reading output from `aws sso login --no-browser --use-device-code` and automatically filling in credentials using browser headless automation with go-rod.

> Use password manager cli to get username, password, 2fa code or totp secret in secure way.

Note that to make it work you need to have `Authenticator app` as a Multi-factor authentication (MFA) device in your AWS account settings. It won't work with passkeys.

## Features

- ✅ Reads AWS SSO output from stdin via pipe
- ✅ Extracts device URL from AWS CLI output
- ✅ Automates browser login with username, password, 2FA code or TOTP secret
- ✅ Supports multiple credential input methods (CLI, environment variables, interactive)
- ✅ TOTP generation from secret key or manual 2FA code input
- ✅ Headless browser mode by default with option to show browser
- ✅ Comprehensive error handling and detailed logging
- ✅ Forwards AWS CLI output to maintain original functionality
- ✅ Configurable timeouts and logging levels
- ✅ Direct device URL input (bypassing AWS CLI pipe)

## How It Works

1. You run `aws sso login --no-browser --use-device-code` and pipe the output to `awsssologin`
2. The tool reads the AWS CLI output from stdin and extracts the device URL
4. The tool opens the device URL in a headless browser and automates the login process
5. Once login is complete, the AWS CLI command finishes successfully

Alternatively, you can provide the device URL directly with `--device-url` to bypass the AWS CLI pipe entirely. Also, it's the only way to use interactive prompts.

## Installation

### Homebrew

```sh
brew install rgeraskin/homebrew/awsssologin
```

### Go

```sh
go install github.com/rgeraskin/awsssologin@latest
```

## Usage

### Basic Usage
```bash
# Get device URL from AWS CLI and keep this command running
aws sso login --no-browser --use-device-code
# Use awsssologin to automate login with interactive prompts
./awsssologin --device-url "https://example.awsapps.com/start/#/device?user_code=ABCD-1234"
```

### With Additional AWS CLI Arguments and Credentials Flags (fully automated)
```bash
aws sso login --sso-session <session-name> --region us-east-1 --no-browser --use-device-code | ./awsssologin -u myusername -p mypassword --2fa 123456
```

### Useful shell alias

Simply add this to your `.zshrc` or `.bashrc` file and login with `asl` command. Use password manager cli to get username, password and totp secret in secure way.

```bash
function asl () {
  aws sso login --sso-session <YOUR SESSION NAME> --no-browser --use-device-code | \
    awsssologin \
      -u $(<YOUR COMMAND TO GET USERNAME>) \
      -p $(<YOUR COMMAND TO GET PASSWORD>) \
      -t $(<YOUR COMMAND TO GET TOTP SECRET>) \
      --timeout 60 \
      $@
}
```

### Command Line Options

| Flag             | Short | Description                                             |
|------------------|-------|---------------------------------------------------------|
| `--username`     | `-u`  | AWS SSO username                                        |
| `--password`     | `-p`  | AWS SSO password                                        |
| `--2fa`          |       | AWS SSO 2FA code                                        |
| `--totp-secret`  | `-t`  | TOTP secret key for automatic 2FA generation            |
| `--device-url`   |       | AWS SSO device URL (if provided, stdin will be ignored) |
| `--show-browser` |       | Show browser window (runs headless by default)          |
| `--timeout`      |       | Timeout in seconds for browser operations (default: 30) |
| `--log-level`    |       | Log level: debug, info, warn, error (default: info)     |
| `--help`         | `-h`  | Show help                                               |

### Credential Priority

Credentials are resolved in the following order (highest to lowest priority):

1. **Command line flags** (`-u`, `-p`, `--2fa`, `--totp-secret`)
2. **Environment variables**:
   - `AWSSSOLOGIN_USERNAME`
   - `AWSSSOLOGIN_PASSWORD`
   - `AWSSSOLOGIN_2FA`
   - `AWSSSOLOGIN_TOTP_SECRET`
3. **Interactive prompts** (works only with `--device-url` flag, no stdin is allowed)

### TOTP Handling

- If `--totp-secret` is provided (or `AWSSSOLOGIN_TOTP_SECRET` env var), TOTP codes are generated automatically
- If no TOTP secret is provided, you'll be prompted to enter the 6-digit code manually
- TOTP secret should be the base32-encoded secret from your authenticator app
- If `--2fa` is provided (or `AWSSSOLOGIN_2FA` env var), it will be used as the 2FA code

### Environment Variables Support

```bash
export AWSSSOLOGIN_USERNAME="your-username"
export AWSSSOLOGIN_PASSWORD="your-password"
export AWSSSOLOGIN_2FA="123456"
export AWSSSOLOGIN_TOTP_SECRET="ABCD1234EFGH5678..."
```

## Browser Automation

The tool uses specific XPath selectors optimized for AWS SSO pages:
- Username field: `//*[@id="awsui-input-0"]`
- Password field: `//*[@id="awsui-input-1"]`
- 2FA input field: `//*[@id="awsui-input-2"]`
- First Allow button: `//*[@id="cli_verification_btn"]`
- Second Allow button: `//*[@data-testid="allow-access-button"]`
- Success message: `//*[@data-analytics-alert="success"]`

Runs in headless mode by default for automated workflows, but can show the browser with `--show-browser` for debugging.

## Troubleshooting

1. **AWS CLI not found**: Ensure AWS CLI is installed and in your PATH
2. **Browser automation fails**: Try running with `--show-browser` to see what's happening
3. **Timeout issues**: Increase timeout with `--timeout 60` (or higher)
4. **Debug information**: Use `--log-level debug` for detailed operation logs
5. **Form fields not found**: The tool tries specific selectors, but some SSO pages may use custom ones. Create an issue if you encounter this.
6. **TOTP issues**: Verify your TOTP secret is correct and properly base32-encoded. Also, if you're using 2FA code, it can expire during the login process. Consider using TOTP secret instead.
7. **No device URL found**: Ensure you're using `--no-browser` and `--use-device-code` flags with `aws sso login`
