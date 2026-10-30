package license

import (
	"strings"
	"testing"
)

func TestLicenseValidation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"service":"mayday","license_key":"DEMO-ONLY","enabled":true}`, true},
		{`{"service":"other","license_key":"DEMO-ONLY","enabled":true}`, false},
		{`{"service":"mayday","license_key":"DEMO-ONLY","enabled":false}`, false},
		{`{"service":"mayday","license_key":"wrong","enabled":true}`, false},
		{`not json`, false},
	} {
		if (validate(strings.NewReader(tc.body)) == nil) != tc.valid {
			t.Errorf("unexpected validation for %s", tc.body)
		}
	}
}
