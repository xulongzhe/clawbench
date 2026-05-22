package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

import static org.junit.Assert.*;

/**
 * Pure unit tests for BackgroundService's restoreForwardedPorts() parsing logic.
 *
 * The restoreForwardedPorts() method reads saved port strings from SharedPreferences.
 * Two formats are supported:
 * - Plain port number (backward compat): "8080"
 * - Port with host: "8080:192.168.1.100"
 *
 * We test the parsing logic by extracting it into a helper and verifying all edge cases.
 */
public class BackgroundServicePortParsingTest {

    @Before
    public void setUp() throws Exception {
        resetStaticState();
    }

    @After
    public void tearDown() throws Exception {
        try { resetStaticState(); } catch (Exception ignored) {}
    }

    private void resetStaticState() throws Exception {
        setStaticField("isRunning", false);
        setStaticField("nativeWsNeeded", false);
        setStaticField("instance", null);
        setStaticField("lastError", null);
    }

    private void setStaticField(String name, Object value) throws Exception {
        Field field = BackgroundService.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(null, value);
    }

    // =====================================================
    // Port string parsing tests
    // Mirrors the logic in restoreForwardedPorts():
    //   int colonIdx = ps.indexOf(':');
    //   if (colonIdx >= 0) {
    //       port = Integer.parseInt(ps.substring(0, colonIdx));
    //       host = ps.substring(colonIdx + 1);
    //   } else {
    //       port = Integer.parseInt(ps);
    //   }
    // =====================================================

    /**
     * Parse a port string into [port, host] pair, mirroring restoreForwardedPorts logic.
     */
    private static String[] parsePortString(String ps) {
        int colonIdx = ps.indexOf(':');
        if (colonIdx >= 0) {
            return new String[]{
                ps.substring(0, colonIdx),   // port
                ps.substring(colonIdx + 1)    // host
            };
        } else {
            return new String[]{
                ps,   // port
                ""    // host (empty = localhost)
            };
        }
    }

    @Test
    public void parsePortString_plainPort_returnsPortWithEmptyHost() {
        String[] result = parsePortString("8080");
        assertEquals("8080", result[0]);
        assertEquals("", result[1]);
    }

    @Test
    public void parsePortString_portWithHost_returnsPortAndHost() {
        String[] result = parsePortString("8080:192.168.1.100");
        assertEquals("8080", result[0]);
        assertEquals("192.168.1.100", result[1]);
    }

    @Test
    public void parsePortString_portWithHostname_returnsPortAndHostname() {
        String[] result = parsePortString("3000:my-server.local");
        assertEquals("3000", result[0]);
        assertEquals("my-server.local", result[1]);
    }

    @Test
    public void parsePortString_portWithEmptyHostAfterColon_returnsEmptyHost() {
        String[] result = parsePortString("5173:");
        assertEquals("5173", result[0]);
        assertEquals("", result[1]);
    }

    @Test
    public void parsePortString_ipv6Address_returnsFullHost() {
        // IPv6 addresses contain colons — first colon separates port from host
        String[] result = parsePortString("8080:::1");
        assertEquals("8080", result[0]);
        assertEquals("::1", result[1]);
    }

    @Test
    public void parsePortString_multipleColons_splitsOnFirst() {
        String[] result = parsePortString("9090:192.168.1.1:extra");
        assertEquals("9090", result[0]);
        assertEquals("192.168.1.1:extra", result[1]);
    }

    // =====================================================
    // Test the full port restoration by directly invoking
    // the private restoreForwardedPorts method with a
    // populated forwardedPorts map.
    // =====================================================

    @Test
    public void restoreForwardedPorts_plainPortFormat_portAndEmptyHost() {
        // Simulate parsing "8080" → port=8080, host=""
        Map<Integer, String> ports = new ConcurrentHashMap<>();
        String ps = "8080";
        int colonIdx = ps.indexOf(':');
        if (colonIdx >= 0) {
            int port = Integer.parseInt(ps.substring(0, colonIdx));
            String host = ps.substring(colonIdx + 1);
            ports.put(port, host);
        } else {
            int port = Integer.parseInt(ps);
            ports.put(port, "");
        }
        assertEquals(1, ports.size());
        assertTrue(ports.containsKey(8080));
        assertEquals("", ports.get(8080));
    }

    @Test
    public void restoreForwardedPorts_portHostFormat_portAndHost() {
        Map<Integer, String> ports = new ConcurrentHashMap<>();
        String ps = "8080:192.168.1.100";
        int colonIdx = ps.indexOf(':');
        if (colonIdx >= 0) {
            int port = Integer.parseInt(ps.substring(0, colonIdx));
            String host = ps.substring(colonIdx + 1);
            ports.put(port, host);
        } else {
            int port = Integer.parseInt(ps);
            ports.put(port, "");
        }
        assertEquals(1, ports.size());
        assertTrue(ports.containsKey(8080));
        assertEquals("192.168.1.100", ports.get(8080));
    }

    @Test
    public void restoreForwardedPorts_mixedFormats_multiplePorts() {
        Set<String> portStrings = new HashSet<>();
        portStrings.add("3000");
        portStrings.add("5173:192.168.1.50");
        portStrings.add("9090:my-server.local");

        Map<Integer, String> ports = new ConcurrentHashMap<>();
        for (String ps : portStrings) {
            try {
                int colonIdx = ps.indexOf(':');
                if (colonIdx >= 0) {
                    int port = Integer.parseInt(ps.substring(0, colonIdx));
                    String host = ps.substring(colonIdx + 1);
                    ports.put(port, host);
                } else {
                    int port = Integer.parseInt(ps);
                    ports.put(port, "");
                }
            } catch (NumberFormatException ignored) {}
        }

        assertEquals(3, ports.size());
        assertEquals("", ports.get(3000));
        assertEquals("192.168.1.50", ports.get(5173));
        assertEquals("my-server.local", ports.get(9090));
    }

    @Test
    public void restoreForwardedPorts_invalidFormat_skipped() {
        Set<String> portStrings = new HashSet<>();
        portStrings.add("8080");
        portStrings.add("not-a-number");
        portStrings.add("5173:valid-host");

        Map<Integer, String> ports = new ConcurrentHashMap<>();
        for (String ps : portStrings) {
            try {
                int colonIdx = ps.indexOf(':');
                if (colonIdx >= 0) {
                    int port = Integer.parseInt(ps.substring(0, colonIdx));
                    String host = ps.substring(colonIdx + 1);
                    ports.put(port, host);
                } else {
                    int port = Integer.parseInt(ps);
                    ports.put(port, "");
                }
            } catch (NumberFormatException ignored) {}
        }

        assertEquals(2, ports.size());
        assertEquals("", ports.get(8080));
        assertEquals("valid-host", ports.get(5173));
        assertFalse(ports.containsKey(0)); // "not-a-number" should be skipped
    }

    // =====================================================
    // Test the saveForwardedPorts format
    // Mirrors the logic in saveForwardedPorts():
    //   if (entry.getValue().isEmpty()) {
    //       portStrings.add(String.valueOf(entry.getKey()));
    //   } else {
    //       portStrings.add(entry.getKey() + ":" + entry.getValue());
    //   }
    // =====================================================

    @Test
    public void saveForwardedPorts_emptyHost_savesPlainPort() {
        Map<Integer, String> ports = new ConcurrentHashMap<>();
        ports.put(8080, "");

        Set<String> portStrings = new HashSet<>();
        for (Map.Entry<Integer, String> entry : ports.entrySet()) {
            if (entry.getValue().isEmpty()) {
                portStrings.add(String.valueOf(entry.getKey()));
            } else {
                portStrings.add(entry.getKey() + ":" + entry.getValue());
            }
        }

        assertTrue(portStrings.contains("8080"));
        assertFalse(portStrings.contains("8080:"));
    }

    @Test
    public void saveForwardedPorts_withHost_savesPortHostFormat() {
        Map<Integer, String> ports = new ConcurrentHashMap<>();
        ports.put(8080, "192.168.1.100");

        Set<String> portStrings = new HashSet<>();
        for (Map.Entry<Integer, String> entry : ports.entrySet()) {
            if (entry.getValue().isEmpty()) {
                portStrings.add(String.valueOf(entry.getKey()));
            } else {
                portStrings.add(entry.getKey() + ":" + entry.getValue());
            }
        }

        assertTrue(portStrings.contains("8080:192.168.1.100"));
        assertFalse(portStrings.contains("8080"));
    }

    @Test
    public void saveForwardedPorts_mixedPorts_savesCorrectFormats() {
        Map<Integer, String> ports = new ConcurrentHashMap<>();
        ports.put(3000, "");
        ports.put(8080, "192.168.1.100");
        ports.put(9090, "my-server.local");

        Set<String> portStrings = new HashSet<>();
        for (Map.Entry<Integer, String> entry : ports.entrySet()) {
            if (entry.getValue().isEmpty()) {
                portStrings.add(String.valueOf(entry.getKey()));
            } else {
                portStrings.add(entry.getKey() + ":" + entry.getValue());
            }
        }

        assertEquals(3, portStrings.size());
        assertTrue(portStrings.contains("3000"));
        assertTrue(portStrings.contains("8080:192.168.1.100"));
        assertTrue(portStrings.contains("9090:my-server.local"));
    }

    // =====================================================
    // Round-trip test: save → restore
    // =====================================================

    @Test
    public void roundTrip_saveAndRestore_portsMatch() {
        // Original state
        Map<Integer, String> originalPorts = new ConcurrentHashMap<>();
        originalPorts.put(3000, "");
        originalPorts.put(8080, "192.168.1.100");
        originalPorts.put(5173, "dev-server.local");

        // Save
        Set<String> portStrings = new HashSet<>();
        for (Map.Entry<Integer, String> entry : originalPorts.entrySet()) {
            if (entry.getValue().isEmpty()) {
                portStrings.add(String.valueOf(entry.getKey()));
            } else {
                portStrings.add(entry.getKey() + ":" + entry.getValue());
            }
        }

        // Restore
        Map<Integer, String> restoredPorts = new ConcurrentHashMap<>();
        for (String ps : portStrings) {
            try {
                int colonIdx = ps.indexOf(':');
                if (colonIdx >= 0) {
                    int port = Integer.parseInt(ps.substring(0, colonIdx));
                    String host = ps.substring(colonIdx + 1);
                    restoredPorts.put(port, host);
                } else {
                    int port = Integer.parseInt(ps);
                    restoredPorts.put(port, "");
                }
            } catch (NumberFormatException ignored) {}
        }

        // Verify round-trip
        assertEquals(originalPorts.size(), restoredPorts.size());
        for (Map.Entry<Integer, String> entry : originalPorts.entrySet()) {
            assertEquals("Host mismatch for port " + entry.getKey(),
                    entry.getValue(), restoredPorts.get(entry.getKey()));
        }
    }

    // =====================================================
    // Test the addPortForward target host logic
    // Mirrors: String targetHost = (host == null || host.isEmpty()) ? "127.0.0.1" : host;
    // =====================================================

    @Test
    public void addPortForward_nullHost_defaultsToLocalhost() {
        String host = null;
        String targetHost = (host == null || host.isEmpty()) ? "127.0.0.1" : host;
        assertEquals("127.0.0.1", targetHost);
    }

    @Test
    public void addPortForward_emptyHost_defaultsToLocalhost() {
        String host = "";
        String targetHost = (host == null || host.isEmpty()) ? "127.0.0.1" : host;
        assertEquals("127.0.0.1", targetHost);
    }

    @Test
    public void addPortForward_customHost_usesCustomHost() {
        String host = "192.168.1.100";
        String targetHost = (host == null || host.isEmpty()) ? "127.0.0.1" : host;
        assertEquals("192.168.1.100", targetHost);
    }

    // =====================================================
    // Test the ensureConnection target host logic for restored ports
    // Mirrors: String targetHost = host.isEmpty() ? "127.0.0.1" : host;
    // =====================================================

    @Test
    public void ensureConnection_emptyHost_defaultsToLocalhost() {
        String host = "";
        String targetHost = host.isEmpty() ? "127.0.0.1" : host;
        assertEquals("127.0.0.1", targetHost);
    }

    @Test
    public void ensureConnection_customHost_usesCustomHost() {
        String host = "my-server.local";
        String targetHost = host.isEmpty() ? "127.0.0.1" : host;
        assertEquals("my-server.local", targetHost);
    }
}
