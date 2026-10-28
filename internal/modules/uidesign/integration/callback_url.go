package integration

import (
	"fmt"
	"net"
	"strings"
)

func ResolveCallbackURL(backendInternalURL, goBackendURL, appPort string) string {
	return resolveCallbackURL(backendInternalURL, goBackendURL, appPort, net.LookupHost)
}

func resolveCallbackURL(backendInternalURL, goBackendURL, appPort string, lookupHost func(string) ([]string, error)) string {
	baseURL := backendInternalURL
	if baseURL == "" {
		baseURL = goBackendURL
	}
	if baseURL == "" {
		if _, err := lookupHost("backend"); err == nil {
			baseURL = "http://backend:8080"
		} else if _, err := lookupHost("diagram_backend_prod"); err == nil {
			baseURL = "http://diagram_backend_prod:8080"
		} else {
			if appPort == "" {
				appPort = "8080"
			}
			baseURL = fmt.Sprintf("http://localhost:%s", appPort)
		}
	}
	return strings.TrimSuffix(baseURL, "/") + "/internal/v1/jobs/callback"
}
