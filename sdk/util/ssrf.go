// Package util — ssrf.go provides SSRF (Server-Side Request Forgery) protection
// utilities shared across web_fetch, web_search, and external tool callbacks.
package util

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// IsPrivateHost determines if a hostname resolves to a private/internal address.
// This is the canonical SSRF protection function used by all outbound HTTP tools.
//
// Checks:
//  1. localhost / *.localhost / *.local reserved names
//  2. IP literal → direct check
//  3. Domain name → DNS resolve, check all resolved IPs
func IsPrivateHost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return true
	}
	if strings.HasSuffix(hostname, ".local") {
		return true
	}

	// IP literal
	if ip := net.ParseIP(hostname); ip != nil {
		return IsPrivateIP(ip)
	}

	// Domain name: DNS resolve and check
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// Resolution failed — conservatively allow (the HTTP layer will error)
		return false
	}
	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return true
		}
	}
	return false
}

// IsPrivateIP checks if an IP belongs to private/reserved/loopback/link-local ranges.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Cloud metadata service: 169.254.169.254 (AWS/GCP/Azure)
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 169 && v4[1] == 254 && v4[2] == 169 && v4[3] == 254 {
			return true
		}
	}
	return false
}

// ValidateCallbackURL validates a URL for safe outbound requests.
// Checks: scheme (http/https only), no credentials, and SSRF protection.
func ValidateCallbackURL(rawURL string) error {
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL too long (>2048 chars)")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https, got %q", u.Scheme)
	}

	// Block URLs with credentials
	if u.User != nil && (u.User.Username() != "" || strings.Contains(rawURL, "@")) {
		return fmt.Errorf("URLs with credentials are not allowed")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL missing hostname")
	}

	// SSRF: block private/internal hosts
	if IsPrivateHost(hostname) {
		return fmt.Errorf("callback URL points to private/internal address (SSRF blocked): %s", hostname)
	}

	return nil
}

// AllowPrivateCallbackURL is like ValidateCallbackURL but allows private hosts.
// Used when the system explicitly needs to call internal services
// (e.g. localhost Feishu bridge). The caller must ensure the URL is trusted.
func AllowPrivateCallbackURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https, got %q", u.Scheme)
	}
	return nil
}
