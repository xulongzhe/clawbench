package platform

import (
	"net"
	"testing"
)

func TestGetOutboundIP_ReturnsNonLoopback(t *testing.T) {
	ip := GetOutboundIP()
	if ip == "" {
		// May be empty in isolated environments (no network)
		t.Log("GetOutboundIP() returned empty string (no outbound route available)")
		return
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Errorf("GetOutboundIP() returned invalid IP %q", ip)
	}
	if parsed.IsLoopback() {
		t.Errorf("GetOutboundIP() returned loopback IP %q, want non-loopback", ip)
	}
}

func TestGetOutboundIP_Format(t *testing.T) {
	ip := GetOutboundIP()
	if ip == "" {
		t.Skip("no outbound IP available in this environment")
	}
	// Should be a valid IPv4 or IPv6 address string
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Errorf("GetOutboundIP() = %q, not a valid IP address", ip)
	}
}
