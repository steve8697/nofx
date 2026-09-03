package utils

import (
	"fmt"
	"net"
	"net/url"
)

// IsPrivateIP checks if an IP address is private or loopback
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return false // Treat non-IPv4 as public for now (or handle IPv6 if needed)
	}

	// Private IP ranges
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsCloudMetadataIP checks if an IP is a known cloud metadata service
func IsCloudMetadataIP(ip net.IP) bool {
	// AWS, GCP, Azure metadata service
	return ip.String() == "169.254.169.254"
}

// IsValidIP checks if an IP is safe to connect to (not private, not metadata)
func IsValidIP(ip net.IP) bool {
	return !IsPrivateIP(ip) && !IsCloudMetadataIP(ip)
}

// ValidateURL checks if a URL is safe to request
// inputURL: the URL to validate
// allowedHosts: map of whitelisted hostnames (optional)
func ValidateURL(inputURL string, allowedHosts map[string]bool) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("URL is empty")
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	// Only allow http and https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	hostname := parsedURL.Hostname()

	// 1. Check strict whitelist if provided
	if len(allowedHosts) > 0 {
		if !allowedHosts[hostname] {
			return "", fmt.Errorf("host not allowed: %s", hostname)
		}
		// If in whitelist, we assume it's safe (skip DNS check to allow internal whitelisted hosts if needed)
		// But usually we still want to resolve it. strict whitelist implies trust.
		return inputURL, nil
	}

	// 2. DNS Resolution to check for SSRF
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return "", fmt.Errorf("failed to resolve host: %v", err)
	}

	for _, ip := range ips {
		if !IsValidIP(ip) {
			return "", fmt.Errorf("blocked potential SSRF to private/metadata IP: %s (%s)", hostname, ip.String())
		}
	}

	return inputURL, nil
}
