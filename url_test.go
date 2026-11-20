package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestContinueReadingStdinSkipsDevicePrompt(t *testing.T) {
	const url = "https://eu-central-1.signin.aws/platform/authorize?request_uri=urn%3Aabc&identitystore_id=d-123"
	skip := func(line string) bool { return isDevicePromptLine(line, url) }

	for _, tc := range []struct {
		name string
		in   string
		skip func(string) bool
		want string
	}{
		{
			// What aws sso login prints after the first (matched) URL today.
			name: "current signin.aws prompt tail",
			in: "\nThen enter the code:\n\nZXCF-DCHG\n\n" +
				"Alternatively, you may visit the following URL which will autofill the code upon loading:\n" +
				url + "\nSuccessfully logged into Start URL: https://example.awsapps.com/start\n",
			skip: skip,
			want: "Successfully logged into Start URL: https://example.awsapps.com/start\n",
		},
		{
			name: "legacy: nothing left of the prompt",
			in:   "Successfully logged into Start URL: https://example.awsapps.com/start\n",
			skip: skip,
			want: "Successfully logged into Start URL: https://example.awsapps.com/start\n",
		},
		{
			name: "filtering stops at first forwarded line",
			in:   "\nfirst real line\n\nZXCF-DCHG\n",
			skip: skip,
			want: "first real line\n\nZXCF-DCHG\n",
		},
		{
			name: "no filter forwards everything",
			in:   "\nThen enter the code:\n",
			skip: nil,
			want: "\nThen enter the code:\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			continueReadingStdin(bufio.NewScanner(strings.NewReader(tc.in)), &out, tc.skip)
			if got := out.String(); got != tc.want {
				t.Errorf("forwarded %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeviceURLPattern(t *testing.T) {
	const (
		legacy  = "https://example.awsapps.com/start/#/device?user_code=ABCD-1234"
		current = "https://eu-central-1.signin.aws/platform/authorize?request_uri=urn%3Aietf%3Aparams%3Aoauth%3Arequest_uri%3AAAAADmtl-_x&identitystore_id=d-9967074dea"
	)

	for _, tc := range []struct {
		name, line, want string
	}{
		{"legacy", legacy, legacy},
		{"current", current, current},
		{"current with surrounding text", "visit: " + current + " now", current},
		{"no URL", "Then enter the code:", ""},
		{"other host", "https://evil.example.com/platform/authorize?request_uri=x", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := deviceURLPattern.FindString(tc.line); got != tc.want {
				t.Errorf("FindString(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}

	for _, u := range []string{legacy, current} {
		if err := validateDeviceURL(u); err != nil {
			t.Errorf("validateDeviceURL(%q) = %v, want nil", u, err)
		}
	}
	if err := validateDeviceURL("x" + current); err == nil {
		t.Errorf("validateDeviceURL accepted a URL with a leading prefix")
	}
}
