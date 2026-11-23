package auth

import "testing"

func TestValidMonitor(t *testing.T) {
	for _, tc := range []struct {
		url     string
		seconds int
		valid   bool
	}{
		{"https://example.com/path?q=1", 5, true}, {"http://example.com", 420, true},
		{"example.com", 5, false}, {"ftp://example.com", 5, false}, {"http://", 5, false},
		{"https://user:pass@example.com", 5, false}, {"https://example.com/#fragment", 5, false},
		{"https://example.com", 0, false}, {"https://example.com", -1, false},
	} {
		if got := validMonitor(tc.url, tc.seconds); got != tc.valid {
			t.Errorf("validMonitor(%q, %d) = %v", tc.url, tc.seconds, got)
		}
	}
}
