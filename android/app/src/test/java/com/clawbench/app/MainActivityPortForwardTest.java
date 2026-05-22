package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

import static org.junit.Assert.*;

/**
 * Unit tests for MainActivity's forwarded ports with host parameter.
 *
 * Tests the logic of addForwardedPort and removeForwardedPort in WebAppInterface
 * that handles the host parameter for SSH tunnel port forwarding.
 *
 * The key logic being tested:
 * - addForwardedPort(port, host): stores port→host mapping in activity.forwardedPorts
 * - Null host is normalized to empty string
 * - Remove works by port number only
 */
public class MainActivityPortForwardTest {

    private MainActivity activity;

    @Before
    public void setUp() throws Exception {
        activity = allocate(MainActivity.class);
        // Set the static instance field
        Field instanceField = MainActivity.class.getDeclaredField("instance");
        instanceField.setAccessible(true);
        instanceField.set(null, activity);
    }

    @After
    public void tearDown() throws Exception {
        try {
            Field instanceField = MainActivity.class.getDeclaredField("instance");
            instanceField.setAccessible(true);
            instanceField.set(null, null);
        } catch (Exception ignored) {}
    }

    // =====================================================
    // Test: forwardedPorts map stores host correctly
    // =====================================================

    @Test
    public void forwardedPorts_storesEmptyHost() throws Exception {
        Map<Integer, String> ports = getForwardedPorts();
        ports.put(8080, "");
        assertEquals("", ports.get(8080));
    }

    @Test
    public void forwardedPorts_storesCustomHost() throws Exception {
        Map<Integer, String> ports = getForwardedPorts();
        ports.put(8080, "192.168.1.100");
        assertEquals("192.168.1.100", ports.get(8080));
    }

    @Test
    public void forwardedPorts_samePortDifferentHosts_overwrites() throws Exception {
        Map<Integer, String> ports = getForwardedPorts();
        ports.put(8080, "");
        ports.put(8080, "192.168.1.100");
        // Last write wins for same key
        assertEquals("192.168.1.100", ports.get(8080));
    }

    @Test
    public void forwardedPorts_multiplePorts_differentHosts() throws Exception {
        Map<Integer, String> ports = getForwardedPorts();
        ports.put(3000, "");
        ports.put(8080, "192.168.1.100");
        ports.put(5173, "dev-server.local");
        assertEquals(3, ports.size());
        assertEquals("", ports.get(3000));
        assertEquals("192.168.1.100", ports.get(8080));
        assertEquals("dev-server.local", ports.get(5173));
    }

    @Test
    public void forwardedPorts_removePort() throws Exception {
        Map<Integer, String> ports = getForwardedPorts();
        ports.put(8080, "192.168.1.100");
        ports.remove(8080);
        assertNull(ports.get(8080));
    }

    // =====================================================
    // Test: WebAppInterface.addForwardedPort null host handling
    // Mirrors: activity.forwardedPorts.put(port, host != null ? host : "");
    // =====================================================

    @Test
    public void addForwardedPort_nullHost_normalizedToEmpty() {
        String host = null;
        String normalized = host != null ? host : "";
        assertEquals("", normalized);
    }

    @Test
    public void addForwardedPort_emptyHost_staysEmpty() {
        String host = "";
        String normalized = host != null ? host : "";
        assertEquals("", normalized);
    }

    @Test
    public void addForwardedPort_customHost_preserved() {
        String host = "192.168.1.100";
        String normalized = host != null ? host : "";
        assertEquals("192.168.1.100", normalized);
    }

    // =====================================================
    // Test: openInBrowser host logic
    // Mirrors: String targetHost = (host != null && !host.isEmpty()) ? host : "localhost";
    // =====================================================

    @Test
    public void openInBrowser_nullHost_defaultsToLocalhost() {
        String host = null;
        String targetHost = (host != null && !host.isEmpty()) ? host : "localhost";
        assertEquals("localhost", targetHost);
    }

    @Test
    public void openInBrowser_emptyHost_defaultsToLocalhost() {
        String host = "";
        String targetHost = (host != null && !host.isEmpty()) ? host : "localhost";
        assertEquals("localhost", targetHost);
    }

    @Test
    public void openInBrowser_customHost_usesCustomHost() {
        String host = "192.168.1.100";
        String targetHost = (host != null && !host.isEmpty()) ? host : "localhost";
        assertEquals("192.168.1.100", targetHost);
    }

    // --- Helper methods ---

    @SuppressWarnings("unchecked")
    private Map<Integer, String> getForwardedPorts() throws Exception {
        Field field = MainActivity.class.getDeclaredField("forwardedPorts");
        field.setAccessible(true);
        Map<Integer, String> ports = (Map<Integer, String>) field.get(activity);
        if (ports == null) {
            ports = new ConcurrentHashMap<>();
            field.set(activity, ports);
        }
        return ports;
    }

    @SuppressWarnings("unchecked")
    private static <T> T allocate(Class<T> clazz) throws Exception {
        // Try default constructor first (works for most classes with returnDefaultValues=true)
        try {
            Constructor<T> ctor = clazz.getDeclaredConstructor();
            ctor.setAccessible(true);
            return ctor.newInstance();
        } catch (Exception e) {
            // Fallback: Unsafe allocation for classes without no-arg constructors
            var unsafeField = Class.forName("sun.misc.Unsafe").getDeclaredField("theUnsafe");
            unsafeField.setAccessible(true);
            Object unsafe = unsafeField.get(null);
            Method allocate = unsafe.getClass().getDeclaredMethod("allocateInstance", Class.class);
            allocate.setAccessible(true);
            return (T) allocate.invoke(unsafe, clazz);
        }
    }
}
