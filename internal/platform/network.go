package platform

import (
	"context"
	"net"
	"time"
)

// outboundDialer is the dialer used by GetOutboundIP.
// Package-level variable for testability.
var outboundDialer = net.Dialer{Timeout: 1 * time.Second}

// GetOutboundIP returns the preferred outbound IP address of this machine
// by attempting a UDP connection to a public DNS server.
// Returns empty string if the IP cannot be determined.
func GetOutboundIP() string {
	conn, err := outboundDialer.DialContext(context.Background(), "udp", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close() //nolint:errcheck // best-effort close on UDP
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	// Skip loopback addresses
	if addr.IP.IsLoopback() {
		return ""
	}
	return addr.IP.String()
}
