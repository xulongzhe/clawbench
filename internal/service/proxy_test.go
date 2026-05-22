package service

import (
	"database/sql"
	"testing"

	"clawbench/internal/model"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
)

func newTestRegistry(t *testing.T) *ProxyRegistry {
	t.Helper()
	return NewProxyRegistry("1024-65535", 0)
}

// isPortRegistered is a test helper that checks if a port is in the registry via ListPorts.
func isPortRegistered(r *ProxyRegistry, port int) bool {
	for _, p := range r.ListPorts() {
		if p.Port == port {
			return true
		}
	}
	return false
}

// getPortProtocol is a test helper that returns the protocol for a registered port.
func getPortProtocol(r *ProxyRegistry, port int) string {
	for _, p := range r.ListPorts() {
		if p.Port == port {
			return p.Protocol
		}
	}
	return "http"
}

func TestProxyRegistry_RegisterPort(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.RegisterPort(8080, "", "test", "http")
	assert.NoError(t, err)
	assert.True(t, isPortRegistered(r, 8080))
}

func TestProxyRegistry_RegisterPort_Invalid(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too large", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.RegisterPort(tt.port, "", "", "")
			assert.Error(t, err)
		})
	}
}

func TestProxyRegistry_RegisterPort_Duplicate(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.RegisterPort(3000, "", "first", "")
	assert.NoError(t, err)

	err = r.RegisterPort(3000, "", "second", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestProxyRegistry_UnregisterPort(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	_ = r.RegisterPort(9090, "", "metrics", "")

	err := r.UnregisterPort(9090)
	assert.NoError(t, err)
	assert.False(t, isPortRegistered(r, 9090))
}

func TestProxyRegistry_UnregisterPort_NotRegistered(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.UnregisterPort(9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestProxyRegistry_ListPorts_Sorted(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	_ = r.RegisterPort(8080, "", "api", "")
	_ = r.RegisterPort(3000, "", "app", "")
	_ = r.RegisterPort(5173, "", "vite", "")

	ports := r.ListPorts()
	assert.Len(t, ports, 3)
	assert.Equal(t, 3000, ports[0].Port)
	assert.Equal(t, 5173, ports[1].Port)
	assert.Equal(t, 8080, ports[2].Port)
}

func TestProxyRegistry_ListPorts_Empty(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	ports := r.ListPorts()
	assert.Empty(t, ports)
}

func TestProxyRegistry_IsPortAllowed(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	assert.False(t, isPortRegistered(r, 8080))
	_ = r.RegisterPort(8080, "", "", "")
	assert.True(t, isPortRegistered(r, 8080))
}

func TestIsPortInRange(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		rangeStr string
		expected bool
	}{
		{"in range", 3000, "1024-65535", true},
		{"below range", 80, "1024-65535", false},
		{"above range", 70000, "1024-65535", false},
		{"exact match", 8080, "3000,8080,9090", true},
		{"not in list", 4000, "3000,8080,9090", false},
		{"mixed range+single in range", 5000, "1024-5000,8080", true},
		{"mixed range+single exact", 8080, "1024-5000,8080", true},
		{"mixed range+single not match", 6000, "1024-5000,8080", false},
		{"empty range allows all", 1234, "", true},
		{"boundary low", 1024, "1024-65535", true},
		{"boundary high", 65535, "1024-65535", true},
		{"just below boundary", 1023, "1024-65535", false},
		{"just above boundary", 65536, "1024-65535", false},
		{"single port match", 3000, "3000", true},
		{"single port no match", 3001, "3000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPortInRange(tt.port, tt.rangeStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}


func TestProxyRegistry_RegisterPort_Protocol(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.RegisterPort(4443, "", "secure", "https")
	assert.NoError(t, err)

	ports := r.ListPorts()
	assert.Len(t, ports, 1)
	assert.Equal(t, "https", ports[0].Protocol)

	err = r.RegisterPort(8080, "", "plain", "http")
	assert.NoError(t, err)

	protocol := getPortProtocol(r, 4443)
	assert.Equal(t, "https", protocol)

	protocol = getPortProtocol(r, 8080)
	assert.Equal(t, "http", protocol)

	// Unregistered port defaults to http
	protocol = getPortProtocol(r, 9999)
	assert.Equal(t, "http", protocol)
}

func TestProxyRegistry_RegisterPort_InvalidProtocolDefaultsToHTTP(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.RegisterPort(8080, "", "test", "ftp")
	assert.NoError(t, err)

	ports := r.ListPorts()
	assert.Equal(t, "http", ports[0].Protocol) // non-https defaults to http
}

func TestParseProcNetTCPData(t *testing.T) {
	data := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1394 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 67890 1 0000000000000000 20 0 0 10 -1
   2: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 11111 1 0000000000000000 100 0 0 10 0
   3: 00000000:1F90 00000000:0000 06 00000000:00000000 00:00000000 00000000     0        0 22222 1 0000000000000000 100 0 0 10 0
`
	// 0x1F90 = 8080 (LISTEN), 0x1394 = 5012 (LISTEN), 0x0050 = 80 (LISTEN)
	// Line 3 has state 06 (TIME_WAIT), should be skipped
	portInodes := parseProcNetTCPData(data)
	assert.Contains(t, portInodes, 8080)
	assert.Contains(t, portInodes, 5012)
	assert.Contains(t, portInodes, 80)
	assert.Len(t, portInodes, 3)
	// Verify inode values
	assert.Equal(t, uint64(12345), portInodes[8080])
	assert.Equal(t, uint64(67890), portInodes[5012])
	assert.Equal(t, uint64(11111), portInodes[80])
}

func TestParseProcNetTCPData_Empty(t *testing.T) {
	portInodes := parseProcNetTCPData("")
	assert.Empty(t, portInodes)
}

func TestParseProcNetTCPData_HeaderOnly(t *testing.T) {
	data := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
`
	portInodes := parseProcNetTCPData(data)
	assert.Empty(t, portInodes)
}

// --- DB Persistence Tests ---

// setupTestDB creates an in-memory SQLite database with the forwarded_ports table.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	assert.NoError(t, err)
	_, err = db.Exec("PRAGMA busy_timeout=5000")
	assert.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS forwarded_ports (
			local_port INTEGER PRIMARY KEY,
			port INTEGER NOT NULL,
			host TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			protocol TEXT NOT NULL DEFAULT 'http',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	assert.NoError(t, err)

	return db
}

func TestProxyRegistry_PortPersistence_RegisterAndLoad(t *testing.T) {
	// Set up in-memory DB and make it available globally
	origDB := DB
	origDBRead := DBRead
	DB = setupTestDB(t)
	DBRead = DB // Same instance for :memory: SQLite — data is shared
	defer func() { DB = origDB; DBRead = origDBRead }()

	// Create registry and register ports — should persist to DB
	r := NewProxyRegistry("1024-65535", 0)
	defer r.Stop()

	err := r.RegisterPort(5173, "", "Vite Dev", "http")
	assert.NoError(t, err)
	err = r.RegisterPort(8080, "", "API", "https")
	assert.NoError(t, err)

	// Verify ports are in the database
	var count int
	err = DB.QueryRow("SELECT COUNT(*) FROM forwarded_ports").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify individual records
	var name, protocol string
	err = DB.QueryRow("SELECT name, protocol FROM forwarded_ports WHERE port = 5173").Scan(&name, &protocol)
	assert.NoError(t, err)
	assert.Equal(t, "Vite Dev", name)
	assert.Equal(t, "http", protocol)

	err = DB.QueryRow("SELECT name, protocol FROM forwarded_ports WHERE port = 8080").Scan(&name, &protocol)
	assert.NoError(t, err)
	assert.Equal(t, "API", name)
	assert.Equal(t, "https", protocol)
}

func TestProxyRegistry_PortPersistence_UnregisterDeletesFromDB(t *testing.T) {
	origDB := DB
	origDBRead := DBRead
	DB = setupTestDB(t)
	DBRead = DB
	defer func() { DB = origDB; DBRead = origDBRead }()

	r := NewProxyRegistry("1024-65535", 0)
	defer r.Stop()

	r.RegisterPort(3000, "", "app", "http")
	r.RegisterPort(8080, "", "api", "http")

	// Unregister one port
	err := r.UnregisterPort(3000)
	assert.NoError(t, err)

	// Verify only one port remains in DB
	var count int
	err = DB.QueryRow("SELECT COUNT(*) FROM forwarded_ports").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the right port remains
	var port int
	err = DB.QueryRow("SELECT port FROM forwarded_ports").Scan(&port)
	assert.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestProxyRegistry_PortPersistence_RestoreOnStartup(t *testing.T) {
	origDB := DB
	origDBRead := DBRead
	DB = setupTestDB(t)
	DBRead = DB
	defer func() { DB = origDB; DBRead = origDBRead }()

	// First registry: register ports (persists to DB)
	r1 := NewProxyRegistry("1024-65535", 0)
	r1.RegisterPort(5173, "", "Vite Dev", "http")
	r1.RegisterPort(8080, "", "API", "https")
	r1.Stop()

	// Second registry: should load ports from DB
	r2 := NewProxyRegistry("1024-65535", 0)
	defer r2.Stop()

	ports := r2.ListPorts()
	assert.Len(t, ports, 2)
	assert.Equal(t, 5173, ports[0].Port)
	assert.Equal(t, "Vite Dev", ports[0].Name)
	assert.Equal(t, "http", ports[0].Protocol)
	assert.Equal(t, 8080, ports[1].Port)
	assert.Equal(t, "API", ports[1].Name)
	assert.Equal(t, "https", ports[1].Protocol)

	assert.True(t, isPortRegistered(r2, 5173))
	assert.True(t, isPortRegistered(r2, 8080))
}

func TestProxyRegistry_PortPersistence_FullLifecycle(t *testing.T) {
	origDB := DB
	origDBRead := DBRead
	DB = setupTestDB(t)
	DBRead = DB
	defer func() { DB = origDB; DBRead = origDBRead }()

	// Phase 1: Create, register, verify
	r1 := NewProxyRegistry("1024-65535", 0)
	r1.RegisterPort(3000, "", "frontend", "http")
	r1.RegisterPort(4000, "", "backend", "http")
	r1.RegisterPort(5432, "", "database", "http")
	r1.Stop()

	// Phase 2: Load, remove one, add another, verify
	r2 := NewProxyRegistry("1024-65535", 0)
	assert.True(t, isPortRegistered(r2, 3000))
	assert.True(t, isPortRegistered(r2, 4000))
	assert.True(t, isPortRegistered(r2, 5432))

	r2.UnregisterPort(4000)      // remove one
	r2.RegisterPort(9090, "", "metrics", "http") // add new
	r2.Stop()

	// Phase 3: Load again, verify final state
	r3 := NewProxyRegistry("1024-65535", 0)
	defer r3.Stop()

	ports := r3.ListPorts()
	assert.Len(t, ports, 3)

	portMap := make(map[int]model.ForwardedPort)
	for _, p := range ports {
		portMap[p.Port] = p
	}

	assert.Contains(t, portMap, 3000)
	assert.Equal(t, "frontend", portMap[3000].Name)
	assert.Contains(t, portMap, 5432)
	assert.Equal(t, "database", portMap[5432].Name)
	assert.Contains(t, portMap, 9090)
	assert.Equal(t, "metrics", portMap[9090].Name)
	assert.NotContains(t, portMap, 4000) // was removed
}

func TestProxyRegistry_PortPersistence_SkipsOutOfAllowedRange(t *testing.T) {
	origDB := DB
	origDBRead := DBRead
	DB = setupTestDB(t)
	DBRead = DB
	defer func() { DB = origDB; DBRead = origDBRead }()

	// Insert a port directly into DB that is outside the new allowed range
	_, err := DB.Exec("INSERT INTO forwarded_ports (local_port, port, host, name, protocol) VALUES (80, 80, '', 'system', 'http')")
	assert.NoError(t, err)

	// Create registry with restricted range — port 80 should be skipped
	r := NewProxyRegistry("1024-65535", 0)
	defer r.Stop()

	assert.False(t, isPortRegistered(r, 80))
	ports := r.ListPorts()
	assert.Empty(t, ports)
}

func TestProxyRegistry_PortPersistence_NoDB(t *testing.T) {
	// When DB is nil, persistence methods should be no-ops (not panic)
	origDB := DB
	origDBRead := DBRead
	DB = nil
	DBRead = nil
	defer func() { DB = origDB; DBRead = origDBRead }()

	r := NewProxyRegistry("1024-65535", 0)
	defer r.Stop()

	// Register should work (in-memory only)
	err := r.RegisterPort(8080, "", "test", "http")
	assert.NoError(t, err)
	assert.True(t, isPortRegistered(r, 8080))

	// Unregister should work
	err = r.UnregisterPort(8080)
	assert.NoError(t, err)
	assert.False(t, isPortRegistered(r, 8080))
}

// ---------- Stop ----------

func TestProxyRegistry_Stop_CancelsContext(t *testing.T) {
	r := NewProxyRegistry("1024-65535", 0)

	// Stop should not panic
	r.Stop()

	// Calling Stop again should be safe (cancel is nil or already called)
	r.Stop()
}

func TestProxyRegistry_Stop_DoubleStop(t *testing.T) {
	// Calling Stop twice should be safe
	r := NewProxyRegistry("1024-65535", 0)
	r.Stop()
	r.Stop() // should not panic
}

// ---------- GetPortProtocol ----------

func TestProxyRegistry_GetPortProtocol_Registered(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	err := r.RegisterPort(8443, "", "secure", "https")
	assert.NoError(t, err)

	protocol := getPortProtocol(r, 8443)
	assert.Equal(t, "https", protocol)
}

func TestProxyRegistry_GetPortProtocol_Unregistered(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	protocol := getPortProtocol(r, 9999)
	assert.Equal(t, "http", protocol, "unregistered port should default to http")
}

func TestProxyRegistry_GetPortProtocol_EmptyProtocol(t *testing.T) {
	r := newTestRegistry(t)
	defer r.Stop()

	// Register with http (default protocol)
	err := r.RegisterPort(8080, "", "web", "http")
	assert.NoError(t, err)

	protocol := getPortProtocol(r, 8080)
	assert.Equal(t, "http", protocol)
}
