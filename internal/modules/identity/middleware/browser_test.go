package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestAllowedBrowserOriginExplicitLoopbackOrigins(test *testing.T) {
	const origins = "http://localhost:5173,http://localhost:5174,http://127.0.0.1:5173,http://127.0.0.1:5174"
	for _, scenario := range []struct {
		origin    string
		fetchSite string
		allowed   bool
	}{
		{"http://localhost:5174", "same-origin", true},
		{"http://127.0.0.1:5174", "same-origin", true},
		{"http://localhost:5173", "same-origin", true},
		{"http://127.0.0.1:5173", "same-origin", true},
		{"http://127.0.0.1:5175", "same-origin", false},
		{"https://127.0.0.1:5174", "same-origin", false},
		{"http://localhost.attacker.test:5174", "cross-site", false},
		{"http://127.0.0.1.attacker.test:5174", "same-origin", false},
		{"http://localhost:5174", "cross-site", false},
		{"https://attacker.test", "same-origin", false},
		{"null", "", false},
	} {
		test.Run(scenario.origin+"/"+scenario.fetchSite, func(test *testing.T) {
			request := httptest.NewRequest("POST", "/api/auth/login", nil)
			request.Header.Set("Origin", scenario.origin)
			request.Header.Set("Sec-Fetch-Site", scenario.fetchSite)
			if actual := AllowedBrowserOrigin(request, origins); actual != scenario.allowed {
				test.Fatalf("origin %q accepted=%v, want %v", scenario.origin, actual, scenario.allowed)
			}
		})
	}
}
