package platform

import (
	"errors"
	"net"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing LocalAddr behavior.
type mockConn struct {
	localAddr net.Addr
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, net.ErrClosed }
func (m *mockConn) Write(b []byte) (n int, err error)  { return 0, net.ErrClosed }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return m.localAddr }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestGetOutboundIP_ReturnsNonLoopback(t *testing.T) {
	ip := GetOutboundIP()
	if ip == "" {
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

func TestGetOutboundIP_DialError(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	dialOutbound = func() (net.Conn, error) {
		return nil, errors.New("dial error")
	}

	ip := GetOutboundIP()
	if ip != "" {
		t.Errorf("GetOutboundIP() = %q, want empty string on dial error", ip)
	}
}

func TestGetOutboundIP_NonUDPAddr(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	// Return a connection whose LocalAddr is a TCPAddr (not *net.UDPAddr)
	dialOutbound = func() (net.Conn, error) {
		return &mockConn{localAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1234}}, nil
	}

	ip := GetOutboundIP()
	if ip != "" {
		t.Errorf("GetOutboundIP() = %q, want empty string when LocalAddr is not *net.UDPAddr", ip)
	}
}

func TestGetOutboundIP_LoopbackAddr(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	// Return a connection with a loopback UDP address
	dialOutbound = func() (net.Conn, error) {
		return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1234}}, nil
	}

	ip := GetOutboundIP()
	if ip != "" {
		t.Errorf("GetOutboundIP() = %q, want empty string for loopback address", ip)
	}
}

func TestGetOutboundIP_ValidIP(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	// Return a connection with a valid non-loopback UDP address
	dialOutbound = func() (net.Conn, error) {
		return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}}, nil
	}

	ip := GetOutboundIP()
	if ip != "192.168.1.100" {
		t.Errorf("GetOutboundIP() = %q, want %q", ip, "192.168.1.100")
	}
}

func TestGetOutboundIP_IPv6Addr(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	dialOutbound = func() (net.Conn, error) {
		return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("fe80::1"), Port: 12345}}, nil
	}

	ip := GetOutboundIP()
	if ip == "" {
		t.Errorf("GetOutboundIP() returned empty for IPv6 link-local address")
	}
}

func TestGetOutboundIP_ContextCanceled(t *testing.T) {
	orig := dialOutbound
	defer func() { dialOutbound = orig }()

	// Simulate a canceled context by returning an error
	dialOutbound = func() (net.Conn, error) {
		return nil, errors.New("context canceled")
	}

	ip := GetOutboundIP()
	if ip != "" {
		t.Errorf("GetOutboundIP() = %q, want empty string on canceled context", ip)
	}
}
