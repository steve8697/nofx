package utils

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if got := IsPrivateIP(ip); got != tt.want {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url          string
		allowedHosts map[string]bool
		wantErr      bool
	}{
		{"https://google.com", nil, false},
		{"http://127.0.0.1/api", nil, true},        // Localhost IP -> Should fail
		{"http://localhost/api", nil, true},        // Localhost domain -> Should fail
		{"http://192.168.1.100/api", nil, true},    // Private IP -> Should fail
		{"http://169.254.169.254/meta", nil, true}, // Cloud Metadata -> Should fail
		{"ftp://google.com", nil, true},            // Wrong scheme -> Should fail
		{"", nil, true},                            // Empty -> Should fail
	}

	for _, tt := range tests {
		_, err := ValidateURL(tt.url, tt.allowedHosts)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURL(%s) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}
