package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"clawbench/internal/model"
	"clawbench/internal/service"
)

// testServerHelper creates and starts an SSH server on a random port for testing.
func testServerHelper(t *testing.T, password string, portReg *service.ProxyRegistry) *Server {
	t.Helper()
	srv := NewServer(model.SSHConfig{Enabled: true, Port: 0}, 0, password, portReg)

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	srv.addr = fmt.Sprintf("127.0.0.1:%d", port)

	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return srv
}

// testSSHClient connects to an SSH server with the given credentials.
func testSSHClient(t *testing.T, addr, user, password string) *gossh.Client {
	t.Helper()
	clientCfg := &gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.Password(password),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := gossh.Dial("tcp", addr, clientCfg)
	if err != nil {
		t.Fatalf("failed to connect to SSH server %s: %v", addr, err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// startEchoServer starts a TCP echo server on a random port and returns the port.
func startEchoServer(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	addr := listener.Addr().String()
	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						c.Write(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { listener.Close() })
	return port
}

func newTestRegistry(t *testing.T) *service.ProxyRegistry {
	t.Helper()
	r := service.NewProxyRegistry(model.ProxyConfig{Enabled: true, AllowedPorts: "1024-65535"}, 0)
	t.Cleanup(func() { r.Stop() })
	return r
}

// --- Connection & Auth Tests ---

func TestSSHServerConnectAndDisconnect(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "test-password", portReg)

	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	if srv.Fingerprint() == "" {
		t.Error("expected non-empty fingerprint")
	}
	if srv.Port() == 0 {
		t.Error("expected non-zero port")
	}

	// Close and verify
	if err := client.Close(); err != nil {
		t.Errorf("failed to close client: %v", err)
	}
}

func TestSSHServerAuthFailure_WrongPassword(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "correct-password", portReg)

	clientCfg := &gossh.ClientConfig{
		User: "clawbench",
		Auth: []gossh.AuthMethod{
			gossh.Password("wrong-password"),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	_, err := gossh.Dial("tcp", srv.addr, clientCfg)
	if err == nil {
		t.Error("expected auth failure for wrong password")
	}
}

func TestSSHServerAuthFailure_WrongUser(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "test-password", portReg)

	clientCfg := &gossh.ClientConfig{
		User: "root",
		Auth: []gossh.AuthMethod{
			gossh.Password("test-password"),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	_, err := gossh.Dial("tcp", srv.addr, clientCfg)
	if err == nil {
		t.Error("expected auth failure for wrong username")
	}
}

// --- Port Forwarding Tests ---

func TestSSHPortForward_AllowedButUnregisteredPortWorks(t *testing.T) {
	// SSH tunnels only check IsPortAllowed (port range), not IsPortRegistered.
	// Registration is for the HTTP reverse-proxy (protocol metadata), not SSH tunnels.
	portReg := newTestRegistry(t)
	echoPort := startEchoServer(t)
	// Deliberately do NOT register echoPort — it's in the allowed range (1024-65535)

	srv := testServerHelper(t, "test-password", portReg)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	conn, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort))
	if err != nil {
		t.Fatalf("expected port forwarding to work for allowed-but-unregistered port %d, got: %v", echoPort, err)
	}
	defer conn.Close()

	testMsg := []byte("hello unregistered port")
	_, err = conn.Write(testMsg)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(buf[:n]) != string(testMsg) {
		t.Errorf("echo mismatch: got %q, want %q", string(buf[:n]), string(testMsg))
	}
}

func TestSSHPortForward_DisallowedPortRejectedByTunnel(t *testing.T) {
	// Create a registry that only allows specific ports
	r := service.NewProxyRegistry(model.ProxyConfig{Enabled: true, AllowedPorts: "3000-4000"}, 0)
	defer r.Stop()

	srv := testServerHelper(t, "test-password", r)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	// Port 9999 is outside the allowed range (3000-4000)
	_, err := client.Dial("tcp", "127.0.0.1:9999")
	if err == nil {
		t.Error("expected port forwarding to be rejected for port outside allowed range")
	}
}

func TestSSHPortForward_RegisteredPortWorks(t *testing.T) {
	portReg := newTestRegistry(t)
	echoPort := startEchoServer(t)
	portReg.RegisterPort(echoPort, "echo", "http")

	srv := testServerHelper(t, "test-password", portReg)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	conn, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort))
	if err != nil {
		t.Fatalf("expected port forwarding to work for registered port %d, got: %v", echoPort, err)
	}
	defer conn.Close()

	testMsg := []byte("hello ssh tunnel")
	_, err = conn.Write(testMsg)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(buf[:n]) != string(testMsg) {
		t.Errorf("echo mismatch: got %q, want %q", string(buf[:n]), string(testMsg))
	}
}

func TestSSHPortForward_MultiplePorts(t *testing.T) {
	portReg := newTestRegistry(t)

	// Start two echo servers
	echoPort1 := startEchoServer(t)
	echoPort2 := startEchoServer(t)
	portReg.RegisterPort(echoPort1, "echo1", "http")
	portReg.RegisterPort(echoPort2, "echo2", "http")

	srv := testServerHelper(t, "test-password", portReg)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	// Forward to port 1
	conn1, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort1))
	if err != nil {
		t.Fatalf("failed to forward to port %d: %v", echoPort1, err)
	}
	defer conn1.Close()

	// Forward to port 2
	conn2, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort2))
	if err != nil {
		t.Fatalf("failed to forward to port %d: %v", echoPort2, err)
	}
	defer conn2.Close()

	// Test both connections
	testMsg1 := []byte("port1")
	conn1.Write(testMsg1)
	buf1 := make([]byte, 1024)
	conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
	n1, _ := conn1.Read(buf1)
	if string(buf1[:n1]) != string(testMsg1) {
		t.Errorf("port1 echo mismatch: got %q", string(buf1[:n1]))
	}

	testMsg2 := []byte("port2")
	conn2.Write(testMsg2)
	buf2 := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
	n2, _ := conn2.Read(buf2)
	if string(buf2[:n2]) != string(testMsg2) {
		t.Errorf("port2 echo mismatch: got %q", string(buf2[:n2]))
	}
}

func TestSSHPortForward_DisallowedPortRejected(t *testing.T) {
	// Create a registry that only allows specific ports
	r := service.NewProxyRegistry(model.ProxyConfig{Enabled: true, AllowedPorts: "3000-4000"}, 0)
	defer r.Stop()

	// RegisterPort should reject a port outside the allowed range
	err := r.RegisterPort(8080, "outside-range", "http")
	if err == nil {
		t.Error("expected RegisterPort to reject port 8080 (outside allowed range 3000-4000)")
	}
	if r.IsPortRegistered(8080) {
		t.Error("port 8080 should not be registered since it's outside allowed range")
	}
}

func TestSSHPortForward_LargeDataTransfer(t *testing.T) {
	portReg := newTestRegistry(t)
	echoPort := startEchoServer(t)
	portReg.RegisterPort(echoPort, "echo", "http")

	srv := testServerHelper(t, "test-password", portReg)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	conn, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort))
	if err != nil {
		t.Fatalf("failed to forward: %v", err)
	}
	defer conn.Close()

	// Send 64KB of data
	largeMsg := make([]byte, 64*1024)
	for i := range largeMsg {
		largeMsg[i] = byte(i % 256)
	}

	_, err = conn.Write(largeMsg)
	if err != nil {
		t.Fatalf("failed to write large data: %v", err)
	}

	// Read back all data
	received := make([]byte, 0, len(largeMsg))
	buf := make([]byte, 32*1024)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for len(received) < len(largeMsg) {
		n, err := conn.Read(buf)
		if n > 0 {
			received = append(received, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	if len(received) != len(largeMsg) {
		t.Errorf("large data mismatch: got %d bytes, want %d", len(received), len(largeMsg))
	}
}

// --- Host Key Tests ---

func TestSSHServer_HostKeyPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "ssh_host_key")

	portReg := newTestRegistry(t)

	// First server: generates and saves host key
	srv1 := NewServer(model.SSHConfig{Enabled: true, Port: 0, HostKey: keyPath}, 0, "test", portReg)

	// Find port
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := listener.Addr().String()
	listener.Close()
	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	srv1.addr = fmt.Sprintf("127.0.0.1:%d", port)

	go srv1.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	fingerprint1 := srv1.Fingerprint()
	srv1.Close()

	// Verify key file was created
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("host key file should have been created")
	}

	// Second server: loads existing host key
	srv2 := NewServer(model.SSHConfig{Enabled: true, Port: 0, HostKey: keyPath}, 0, "test", portReg)
	listener2, _ := net.Listen("tcp", "127.0.0.1:0")
	addr2 := listener2.Addr().String()
	listener2.Close()
	_, portStr2, _ := net.SplitHostPort(addr2)
	fmt.Sscanf(portStr2, "%d", &port)
	srv2.addr = fmt.Sprintf("127.0.0.1:%d", port)

	go srv2.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	fingerprint2 := srv2.Fingerprint()
	srv2.Close()

	// Fingerprints should match (same host key)
	if fingerprint1 != fingerprint2 {
		t.Errorf("fingerprints should match for persisted key: first=%s, second=%s", fingerprint1, fingerprint2)
	}
}

func TestSSHServer_EphemeralKeyChangesOnRestart(t *testing.T) {
	portReg := newTestRegistry(t)

	// First server with ephemeral key
	srv1 := NewServer(model.SSHConfig{Enabled: true, Port: 0}, 0, "test", portReg)
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := listener.Addr().String()
	listener.Close()
	_, portStr, _ := net.SplitHostPort(addr)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	srv1.addr = fmt.Sprintf("127.0.0.1:%d", port)

	go srv1.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	fp1 := srv1.Fingerprint()
	srv1.Close()

	// Second server with ephemeral key (should be different)
	srv2 := NewServer(model.SSHConfig{Enabled: true, Port: 0}, 0, "test", portReg)
	listener2, _ := net.Listen("tcp", "127.0.0.1:0")
	addr2 := listener2.Addr().String()
	listener2.Close()
	_, portStr2, _ := net.SplitHostPort(addr2)
	fmt.Sscanf(portStr2, "%d", &port)
	srv2.addr = fmt.Sprintf("127.0.0.1:%d", port)

	go srv2.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	fp2 := srv2.Fingerprint()
	srv2.Close()

	// Ephemeral keys should be different across restarts
	if fp1 == fp2 {
		t.Error("ephemeral host keys should differ between server instances")
	}
}

// --- Auto Port Tests ---

func TestSSHServer_AutoPortAssignment(t *testing.T) {
	portReg := newTestRegistry(t)

	// Port 0 should auto-assign to mainPort+1
	srv := NewServer(model.SSHConfig{Enabled: true, Port: 0}, 20000, "test", portReg)
	if srv.Port() != 20001 {
		t.Errorf("expected auto-assigned port 20001, got %d", srv.Port())
	}

	// Explicit port should be used as-is
	srv2 := NewServer(model.SSHConfig{Enabled: true, Port: 22222}, 20000, "test", portReg)
	if srv2.Port() != 22222 {
		t.Errorf("expected explicit port 22222, got %d", srv2.Port())
	}
}

// --- Connection Stats Tests ---

func TestSSHServer_ConnectionStats_NoConnections(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "test-password", portReg)

	stats := srv.ConnectionStats()
	if stats.Connected {
		t.Error("expected Connected=false when no clients are connected")
	}
	if stats.ClientCount != 0 {
		t.Errorf("expected ClientCount=0, got %d", stats.ClientCount)
	}
	if stats.ActiveChannels != 0 {
		t.Errorf("expected ActiveChannels=0, got %d", stats.ActiveChannels)
	}
	if stats.LastConnectedAt != "" {
		t.Errorf("expected empty LastConnectedAt, got %q", stats.LastConnectedAt)
	}
}

func TestSSHServer_ConnectionStats_ClientConnected(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "test-password", portReg)

	// Before connecting
	stats := srv.ConnectionStats()
	if stats.Connected {
		t.Error("expected Connected=false before client connects")
	}

	// Connect a client
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	// Give the server a moment to update stats
	time.Sleep(50 * time.Millisecond)

	stats = srv.ConnectionStats()
	if !stats.Connected {
		t.Error("expected Connected=true after client connects")
	}
	if stats.ClientCount != 1 {
		t.Errorf("expected ClientCount=1, got %d", stats.ClientCount)
	}
	if stats.LastConnectedAt == "" {
		t.Error("expected non-empty LastConnectedAt after client connects")
	}

	// Disconnect
	client.Close()

	// Wait for server to detect disconnect
	time.Sleep(200 * time.Millisecond)

	stats = srv.ConnectionStats()
	if stats.Connected {
		t.Error("expected Connected=false after client disconnects")
	}
	if stats.ClientCount != 0 {
		t.Errorf("expected ClientCount=0 after disconnect, got %d", stats.ClientCount)
	}
}

func TestSSHServer_ConnectionStats_MultipleClients(t *testing.T) {
	portReg := newTestRegistry(t)
	srv := testServerHelper(t, "test-password", portReg)

	client1 := testSSHClient(t, srv.addr, "clawbench", "test-password")
	client2 := testSSHClient(t, srv.addr, "clawbench", "test-password")

	time.Sleep(50 * time.Millisecond)

	stats := srv.ConnectionStats()
	if stats.ClientCount != 2 {
		t.Errorf("expected ClientCount=2, got %d", stats.ClientCount)
	}

	client1.Close()
	time.Sleep(200 * time.Millisecond)

	stats = srv.ConnectionStats()
	if stats.ClientCount != 1 {
		t.Errorf("expected ClientCount=1 after one disconnect, got %d", stats.ClientCount)
	}

	client2.Close()
	time.Sleep(200 * time.Millisecond)

	stats = srv.ConnectionStats()
	if stats.Connected {
		t.Error("expected Connected=false after all clients disconnect")
	}
}

func TestSSHServer_ConnectionStats_ActiveChannels(t *testing.T) {
	portReg := newTestRegistry(t)
	echoPort := startEchoServer(t)
	portReg.RegisterPort(echoPort, "echo", "http")

	srv := testServerHelper(t, "test-password", portReg)
	client := testSSHClient(t, srv.addr, "clawbench", "test-password")

	time.Sleep(50 * time.Millisecond)

	// Before opening any channels
	stats := srv.ConnectionStats()
	if stats.ActiveChannels != 0 {
		t.Errorf("expected ActiveChannels=0 before port forward, got %d", stats.ActiveChannels)
	}

	// Open a port forward channel
	conn, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", echoPort))
	if err != nil {
		t.Fatalf("failed to dial forwarded port: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	stats = srv.ConnectionStats()
	if stats.ActiveChannels < 1 {
		t.Errorf("expected ActiveChannels>=1 after opening channel, got %d", stats.ActiveChannels)
	}

	// Close the forwarded connection
	conn.Close()
	time.Sleep(200 * time.Millisecond)

	stats = srv.ConnectionStats()
	if stats.ActiveChannels != 0 {
		t.Errorf("expected ActiveChannels=0 after closing channel, got %d", stats.ActiveChannels)
	}
}
