package service

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clawbench/internal/model"
)

// ProxyRegistry manages forwarded ports: registration, health checks, and auto-detection.
type ProxyRegistry struct {
	mu       sync.RWMutex
	ports    map[int]*model.ForwardedPort
	cfg      model.ProxyConfig
	selfPort int // ClawBench's own port, excluded from detection
	cancel   context.CancelFunc
}

// ProxyService is the global singleton, initialized from main.go.
var ProxyService *ProxyRegistry

// NewProxyRegistry creates a new port registry and starts background health checks.
// It also restores any previously persisted ports from the database.
func NewProxyRegistry(cfg model.ProxyConfig, selfPort int) *ProxyRegistry {
	if !cfg.Enabled {
		slog.Info("proxy service disabled by config")
		return &ProxyRegistry{
			ports:    make(map[int]*model.ForwardedPort),
			cfg:      cfg,
			selfPort: selfPort,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &ProxyRegistry{
		ports:    make(map[int]*model.ForwardedPort),
		cfg:      cfg,
		selfPort: selfPort,
		cancel:   cancel,
	}

	// Set default allowed ports
	if cfg.AllowedPorts == "" {
		r.cfg.AllowedPorts = "1024-65535"
	}

	// Restore persisted ports from database
	r.loadPortsFromDB()

	slog.Info("proxy service initialized",
		slog.String("allowed_ports", r.cfg.AllowedPorts),
		slog.Int("self_port", selfPort),
		slog.Int("restored_ports", len(r.ports)),
	)

	// Start background health checker
	go r.healthCheckLoop(ctx)

	return r
}

// Stop shuts down the proxy registry and all health check goroutines.
func (r *ProxyRegistry) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	slog.Info("proxy service stopped")
}

// RegisterPort adds a port to the forwarding registry.
func (r *ProxyRegistry) RegisterPort(port int, name string, protocol string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}
	if !r.IsPortAllowed(port) {
		return fmt.Errorf("port %d is not in the allowed range", port)
	}
	if protocol != "https" {
		protocol = "http"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.ports[port]; exists {
		return fmt.Errorf("port %d is already registered", port)
	}

	r.ports[port] = &model.ForwardedPort{
		Port:       port,
		Name:       name,
		Protocol:   protocol,
		AutoDetect: false,
		Active:     false, // will be updated by health check
	}

	// Persist to database
	r.savePortToDB(port, name, protocol)

	slog.Info("proxy port registered",
		slog.Int("port", port),
		slog.String("name", name),
		slog.String("protocol", protocol),
	)

	return nil
}

// UnregisterPort removes a port from the forwarding registry.
func (r *ProxyRegistry) UnregisterPort(port int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.ports[port]; !exists {
		return fmt.Errorf("port %d is not registered", port)
	}

	delete(r.ports, port)

	// Remove from database
	r.deletePortFromDB(port)

	slog.Info("proxy port unregistered", slog.Int("port", port))
	return nil
}

// ListPorts returns all registered ports with current health status.
func (r *ProxyRegistry) ListPorts() []model.ForwardedPort {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]model.ForwardedPort, 0, len(r.ports))
	for _, p := range r.ports {
		result = append(result, *p)
	}

	// Sort by port number for stable output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Port < result[j].Port
	})

	return result
}

// IsPortAllowed checks whether a port falls within the configured allowed range.
func (r *ProxyRegistry) IsPortAllowed(port int) bool {
	return isPortInRange(port, r.cfg.AllowedPorts)
}

// IsPortRegistered checks whether a port has been explicitly registered.
func (r *ProxyRegistry) IsPortRegistered(port int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.ports[port]
	return exists
}

// GetPortProtocol returns the protocol for a registered port, defaults to "http".
func (r *ProxyRegistry) GetPortProtocol(port int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.ports[port]; ok && p.Protocol != "" {
		return p.Protocol
	}
	return "http"
}

// DetectedPort represents an auto-detected listening port with its protocol.
type DetectedPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "http" or "https"
}

// DetectListeningPorts returns a list of TCP ports currently in LISTEN state
// on the server, filtered to exclude system ports and ClawBench's own port.
// Each port is probed to determine if it speaks TLS (https).
func (r *ProxyRegistry) DetectListeningPorts() []DetectedPort {
	var ports []int

	switch runtime.GOOS {
	case "linux":
		ports = parseProcNetTCP()
	case "darwin":
		ports = parseLsof()
	case "windows":
		ports = parseNetstat()
	default:
		slog.Warn("port auto-detection not supported on this OS", slog.String("os", runtime.GOOS))
		return nil
	}

	// Filter: exclude system ports (< 1024) and our own port
	filtered := make([]int, 0, len(ports))
	for _, p := range ports {
		if p >= 1024 && p != r.selfPort {
			filtered = append(filtered, p)
		}
	}

	sort.Ints(filtered)

	// Probe each port for TLS
	result := make([]DetectedPort, len(filtered))
	for i, p := range filtered {
		protocol := "http"
		if detectTLS(p) {
			protocol = "https"
		}
		result[i] = DetectedPort{Port: p, Protocol: protocol}
	}
	return result
}

// detectTLS attempts a TLS handshake to determine if a port speaks HTTPS.
func detectTLS(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	err = tlsConn.Handshake()
	return err == nil
}

// healthCheckLoop periodically checks if registered ports are still listening.
func (r *ProxyRegistry) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Run an immediate check
	r.checkAllPorts()

	for {
		select {
		case <-ticker.C:
			r.checkAllPorts()
		case <-ctx.Done():
			return
		}
	}
}

// checkAllPorts dials each registered port to determine if it's active.
func (r *ProxyRegistry) checkAllPorts() {
	r.mu.RLock()
	portList := make([]int, 0, len(r.ports))
	for port := range r.ports {
		portList = append(portList, port)
	}
	r.mu.RUnlock()

	for _, port := range portList {
		active := checkPortActive(port)

		r.mu.Lock()
		if p, ok := r.ports[port]; ok {
			p.Active = active
		}
		r.mu.Unlock()
	}
}

// checkPortActive attempts a TCP connection to determine if a port is listening.
func checkPortActive(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// parseProcNetTCP reads /proc/net/tcp on Linux to find LISTEN sockets.
func parseProcNetTCP() []int {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		slog.Debug("failed to read /proc/net/tcp", slog.String("err", err.Error()))
		return nil
	}

	return parseProcNetTCPData(string(data))
}

// parseProcNetTCPData parses the content of /proc/net/tcp.
// Format: sl local_address local_port state ...
// local_address is hex IP, local_port is hex port, state 0A = LISTEN
func parseProcNetTCPData(data string) []int {
	var ports []int
	scanner := bufio.NewScanner(strings.NewReader(data))

	// Skip header line
	if !scanner.Scan() {
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// local_address:port is in field[1], e.g. "00000000:1F90"
		localAddr := fields[1]
		colonIdx := strings.LastIndex(localAddr, ":")
		if colonIdx < 0 {
			continue
		}
		portHex := localAddr[colonIdx+1:]

		// state is in field[3], "0A" = LISTEN
		state := fields[3]
		if state != "0A" {
			continue
		}

		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil {
			continue
		}

		ports = append(ports, int(port))
	}

	return ports
}

// parseLsof uses lsof on macOS to find LISTEN sockets.
func parseLsof() []int {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("lsof command failed", slog.String("err", err.Error()))
		return nil
	}

	var ports []int
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		// Lines look like: node  12345 user  23u  IPv4 ...  TCP *:5173 (LISTEN)
		// We want the :PORT part
		if idx := strings.LastIndex(line, ":"); idx >= 0 {
			rest := line[idx+1:]
			// Extract port number (stop at space or paren)
			var portStr string
			for _, ch := range rest {
				if ch >= '0' && ch <= '9' {
					portStr += string(ch)
				} else {
					break
				}
			}
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				ports = append(ports, port)
			}
		}
	}

	// Deduplicate
	seen := make(map[int]bool)
	unique := make([]int, 0)
	for _, p := range ports {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	return unique
}

// parseNetstat uses netstat on Windows to find LISTEN sockets.
func parseNetstat() []int {
	cmd := exec.Command("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("netstat command failed", slog.String("err", err.Error()))
		return nil
	}

	var ports []int
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		// Lines look like:  TCP    0.0.0.0:5173          0.0.0.0:0              LISTENING       12345
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		localAddr := fields[1]
		colonIdx := strings.LastIndex(localAddr, ":")
		if colonIdx < 0 {
			continue
		}
		port, err := strconv.Atoi(localAddr[colonIdx+1:])
		if err == nil && port > 0 {
			ports = append(ports, port)
		}
	}

	seen := make(map[int]bool)
	unique := make([]int, 0)
	for _, p := range ports {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	return unique
}

// isPortInRange checks if a port number falls within the allowed range string.
// Supported formats: "1024-65535", "3000,5173,8080", "1024-5000,8080"
func isPortInRange(port int, rangeStr string) bool {
	if rangeStr == "" {
		return true // empty = allow all
	}

	parts := strings.Split(rangeStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) != 2 {
				continue
			}
			low, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			high, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil {
				continue
			}
			if port >= low && port <= high {
				return true
			}
		} else {
			p, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			if port == p {
				return true
			}
		}
	}

	return false
}

// loadPortsFromDB restores previously persisted forwarded ports from the database.
// Called once during ProxyRegistry initialization.
func (r *ProxyRegistry) loadPortsFromDB() {
	if DB == nil {
		return
	}

	rows, err := DB.Query("SELECT port, name, protocol FROM forwarded_ports")
	if err != nil {
		slog.Warn("failed to load persisted ports from DB", slog.String("err", err.Error()))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var port int
		var name, protocol string
		if err := rows.Scan(&port, &name, &protocol); err != nil {
			continue
		}
		if !r.IsPortAllowed(port) {
			slog.Warn("skipping persisted port outside allowed range", slog.Int("port", port))
			continue
		}
		if protocol != "https" {
			protocol = "http"
		}
		r.ports[port] = &model.ForwardedPort{
			Port:       port,
			Name:       name,
			Protocol:   protocol,
			AutoDetect: false,
			Active:     false, // will be updated by health check
		}
	}

	if len(r.ports) > 0 {
		slog.Info("restored forwarded ports from database", slog.Int("count", len(r.ports)))
	}
}

// savePortToDB persists a single forwarded port to the database.
func (r *ProxyRegistry) savePortToDB(port int, name, protocol string) {
	if DB == nil {
		return
	}
	_, err := DB.Exec(
		"INSERT OR REPLACE INTO forwarded_ports (port, name, protocol) VALUES (?, ?, ?)",
		port, name, protocol,
	)
	if err != nil {
		slog.Error("failed to persist port to DB", slog.Int("port", port), slog.String("err", err.Error()))
	}
}

// deletePortFromDB removes a forwarded port from the database.
func (r *ProxyRegistry) deletePortFromDB(port int) {
	if DB == nil {
		return
	}
	_, err := DB.Exec("DELETE FROM forwarded_ports WHERE port = ?", port)
	if err != nil {
		slog.Error("failed to delete port from DB", slog.Int("port", port), slog.String("err", err.Error()))
	}
}
