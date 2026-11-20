package main

import "testing"

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
