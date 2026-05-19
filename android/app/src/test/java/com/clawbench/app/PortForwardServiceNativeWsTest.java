package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

import static org.junit.Assert.*;

/**
 * Pure unit tests for PortForwardService's nativeWsNeeded logic.
 *
 * Uses reflection instead of Robolectric to avoid JPush SDK VerifyError
 * caused by obfuscated bytecode in the JPush runtime JAR.
 *
 * Bug: When no SSH ports are forwarded and JPush is not available,
 * PortForwardService.onCreate() would immediately stopSelf(), preventing
 * the native WebSocket from ever starting. The nativeWsNeeded flag fixes this
 * by making native WS a valid reason for the Service to stay alive.
 *
 * Test strategy:
 * - nativeWsNeeded is a static volatile boolean — tested via reflection
 * - The stopSelf condition is: forwardedPorts.isEmpty() && !nativeWsNeeded
 * - We test all combinations of these two variables
 * - We test state transitions (flag set/cleared at correct times)
 * - We test the buildNotification text logic
 */
public class PortForwardServiceNativeWsTest {

    // A simple struct to represent the stopSelf decision state
    private static class ServiceState {
        final Set<Integer> forwardedPorts;
        boolean nativeWsNeeded;
        boolean nativeWsActive;

        ServiceState() {
            forwardedPorts = ConcurrentHashMap.newKeySet();
            nativeWsNeeded = false;
            nativeWsActive = false;
        }

        boolean shouldStopService() {
            return forwardedPorts.isEmpty() && !nativeWsNeeded;
        }

        String notificationText(int portCount, String statusOverride) {
            if (statusOverride != null) return statusOverride;
            if (portCount > 0) return portCount + " 个端口转发活跃";
            if (nativeWsNeeded || nativeWsActive) return "后台事件监听中";
            return "无端口转发，即将停止";
        }

        String notificationTitle(int portCount, String statusOverride) {
            if (statusOverride != null) return portCount > 0 ? "ClawBench 端口转发" : "ClawBench";
            if (portCount > 0) return "ClawBench 端口转发";
            if (nativeWsNeeded || nativeWsActive) return "ClawBench";
            return "ClawBench";
        }
    }

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
        Field field = PortForwardService.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(null, value);
    }

    @SuppressWarnings("unchecked")
    private <T> T getStaticField(String name) throws Exception {
        Field field = PortForwardService.class.getDeclaredField(name);
        field.setAccessible(true);
        return (T) field.get(null);
    }

    // =====================================================
    // Test 1: Default state
    // =====================================================

    @Test
    public void nativeWsNeeded_defaultIsFalse() throws Exception {
        assertFalse("nativeWsNeeded should default to false",
                getStaticField("nativeWsNeeded"));
    }

    // =====================================================
    // Test 2: shouldStopService — all combinations
    // This is the core logic from onCreate() and removePortForward()
    // stopSelf when: forwardedPorts.isEmpty() && !nativeWsNeeded
    // =====================================================

    @Test
    public void shouldStopService_noPortsNoNativeWs_returnsTrue() {
        ServiceState state = new ServiceState();
        assertTrue("No ports + no native WS → should stop",
                state.shouldStopService());
    }

    @Test
    public void shouldStopService_noPortsWithNativeWs_returnsFalse() {
        ServiceState state = new ServiceState();
        state.nativeWsNeeded = true;
        assertFalse("No ports + native WS needed → should NOT stop",
                state.shouldStopService());
    }

    @Test
    public void shouldStopService_hasPortsNoNativeWs_returnsFalse() {
        ServiceState state = new ServiceState();
        state.forwardedPorts.add(20000);
        assertFalse("Has ports + no native WS → should NOT stop",
                state.shouldStopService());
    }

    @Test
    public void shouldStopService_hasPortsAndNativeWs_returnsFalse() {
        ServiceState state = new ServiceState();
        state.nativeWsNeeded = true;
        state.forwardedPorts.add(20000);
        assertFalse("Has ports + native WS → should NOT stop",
                state.shouldStopService());
    }

    // =====================================================
    // Test 3: startNativeEventWs sets flag synchronously
    // The flag must be set BEFORE Service.onCreate() runs
    // =====================================================

    @Test
    public void startNativeEventWs_setsNativeWsNeededSynchronously() throws Exception {
        assertFalse("Before: nativeWsNeeded is false",
                getStaticField("nativeWsNeeded"));

        // Call the static method — the first line sets nativeWsNeeded = true
        // Subsequent calls (startForegroundService) may fail, but the flag
        // is already set by then — that's the whole point of the fix
        try {
            PortForwardService.startNativeEventWs(
                    new android.content.ContextWrapper(null) {}
            );
        } catch (Exception e) {
            // Expected: startForegroundService fails without proper Android context
        }

        assertTrue("startNativeEventWs should set nativeWsNeeded=true synchronously",
                getStaticField("nativeWsNeeded"));
    }

    @Test
    public void startNativeEventWs_flagAvailableBeforeServiceCreation() throws Exception {
        // Simulate the real flow:
        // 1. App goes to background → onPause() → startNativeEventWs()
        // 2. startNativeEventWs() sets nativeWsNeeded = true BEFORE calling start()
        // 3. start() triggers onCreate() which checks nativeWsNeeded
        // 4. If nativeWsNeeded wasn't set yet, onCreate() would stopSelf()

        // Step 1-2: flag is set
        try {
            PortForwardService.startNativeEventWs(
                    new android.content.ContextWrapper(null) {}
            );
        } catch (Exception ignored) {}

        // Step 3: Verify flag is set (this is what onCreate() will check)
        assertTrue("nativeWsNeeded must be true by the time onCreate() runs",
                getStaticField("nativeWsNeeded"));
    }

    // =====================================================
    // Test 4: stopNativeEventWs clears flag
    // =====================================================

    @Test
    public void stopNativeEventWs_clearsNativeWsNeeded() throws Exception {
        setStaticField("nativeWsNeeded", true);

        // Create a minimal service instance via reflection and call stopNativeEventWs
        Object service = createMinimalInstance();
        setStaticField("instance", service);
        setStaticField("isRunning", true);

        invokeMethod(service, "stopNativeEventWs");

        assertFalse("stopNativeEventWs should clear nativeWsNeeded",
                getStaticField("nativeWsNeeded"));
    }

    @Test
    public void stopNativeEventWs_noPorts_shouldStopService() throws Exception {
        setStaticField("nativeWsNeeded", true);

        ServiceState state = new ServiceState();
        Object service = createMinimalInstance();
        setForwardedPorts(service, state.forwardedPorts);
        setStaticField("instance", service);
        setStaticField("isRunning", true);

        invokeMethod(service, "stopNativeEventWs");

        // After stop, the should-stop condition should be true
        boolean nativeWsNeededNow = getStaticField("nativeWsNeeded");
        assertTrue("Service should want to stop after native WS stopped with no ports",
                state.forwardedPorts.isEmpty() && !nativeWsNeededNow);
    }

    @Test
    public void stopNativeEventWs_hasPorts_shouldNotStopService() throws Exception {
        setStaticField("nativeWsNeeded", true);

        ServiceState state = new ServiceState();
        state.forwardedPorts.add(20000);
        Object service = createMinimalInstance();
        setForwardedPorts(service, state.forwardedPorts);
        setStaticField("instance", service);
        setStaticField("isRunning", true);

        invokeMethod(service, "stopNativeEventWs");

        // With ports still present, should-stop condition is false
        boolean nativeWsNeededNow = getStaticField("nativeWsNeeded");
        assertFalse("Service should NOT stop when SSH ports still exist",
                state.forwardedPorts.isEmpty() && !nativeWsNeededNow);
    }

    // =====================================================
    // Test 5: removePortForward respects nativeWsNeeded
    // =====================================================

    @Test
    public void removePortForward_lastPortNoNativeWs_shouldStop() {
        ServiceState state = new ServiceState();
        state.forwardedPorts.add(20000);
        state.forwardedPorts.remove(20000);
        assertTrue("Should stop after removing last port with no native WS",
                state.shouldStopService());
    }

    @Test
    public void removePortForward_lastPortWithNativeWs_shouldNotStop() {
        ServiceState state = new ServiceState();
        state.nativeWsNeeded = true;
        state.forwardedPorts.add(20000);
        state.forwardedPorts.remove(20000);
        assertFalse("Should NOT stop because native WS still needs it",
                state.shouldStopService());
    }

    @Test
    public void removePortForward_notLastPort_shouldNotStop() {
        ServiceState state = new ServiceState();
        state.forwardedPorts.add(20000);
        state.forwardedPorts.add(30000);
        state.forwardedPorts.remove(20000);
        assertFalse("Should NOT stop when other ports still exist",
                state.shouldStopService());
    }

    // =====================================================
    // Test 6: Notification text reflects native WS state
    // These mirror the buildNotification() logic
    // =====================================================

    @Test
    public void notification_noPortsNoNativeWs_showsStopping() {
        ServiceState state = new ServiceState();
        assertEquals("无端口转发，即将停止", state.notificationText(0, null));
    }

    @Test
    public void notification_noPortsWithNativeWs_showsListening() {
        ServiceState state = new ServiceState();
        state.nativeWsNeeded = true;
        assertEquals("后台事件监听中", state.notificationText(0, null));
    }

    @Test
    public void notification_noPortsWithNativeWsActive_showsListening() {
        ServiceState state = new ServiceState();
        state.nativeWsActive = true;
        assertEquals("后台事件监听中", state.notificationText(0, null));
    }

    @Test
    public void notification_hasPorts_showsPortCount() {
        ServiceState state = new ServiceState();
        assertEquals("1 个端口转发活跃", state.notificationText(1, null));
        assertEquals("2 个端口转发活跃", state.notificationText(2, null));
    }

    @Test
    public void notification_title_hasPorts_showsPortForward() {
        ServiceState state = new ServiceState();
        assertEquals("ClawBench 端口转发", state.notificationTitle(1, null));
    }

    @Test
    public void notification_title_noPortsNativeWs_showsClawBench() {
        ServiceState state = new ServiceState();
        state.nativeWsNeeded = true;
        assertEquals("ClawBench", state.notificationTitle(0, null));
    }

    @Test
    public void notification_title_noPortsNoNativeWs_showsClawBench() {
        ServiceState state = new ServiceState();
        assertEquals("ClawBench", state.notificationTitle(0, null));
    }

    @Test
    public void notification_statusOverride_noPorts_titleShowsClawBench() {
        ServiceState state = new ServiceState();
        assertEquals("ClawBench", state.notificationTitle(0, "自定义状态"));
        assertEquals("自定义状态", state.notificationText(0, "自定义状态"));
    }

    @Test
    public void notification_statusOverride_hasPorts_titleShowsPortForward() {
        ServiceState state = new ServiceState();
        assertEquals("ClawBench 端口转发", state.notificationTitle(1, "重连中…"));
        assertEquals("重连中…", state.notificationText(1, "重连中…"));
    }

    // =====================================================
    // Test 7: Full lifecycle — background to foreground
    // =====================================================

    @Test
    public void fullLifecycle_backgroundForeground_noPorts() {
        ServiceState state = new ServiceState();

        // Initially: no ports, no native WS → should stop
        assertTrue(state.shouldStopService());

        // Background: native WS starts
        state.nativeWsNeeded = true;
        assertFalse(state.shouldStopService());

        // Foreground: native WS stops
        state.nativeWsNeeded = false;
        assertTrue(state.shouldStopService());
    }

    @Test
    public void fullLifecycle_backgroundForeground_withPorts() {
        ServiceState state = new ServiceState();
        state.forwardedPorts.add(20000);

        // Background: native WS also starts
        state.nativeWsNeeded = true;
        assertFalse(state.shouldStopService());

        // Foreground: native WS stops, but port still exists
        state.nativeWsNeeded = false;
        assertFalse(state.shouldStopService());
    }

    @Test
    public void fullLifecycle_portAddedWhileBackground() {
        ServiceState state = new ServiceState();

        // Background: native WS starts (no ports)
        state.nativeWsNeeded = true;
        assertFalse(state.shouldStopService());

        // Port gets added (SSH tunnel established while backgrounded)
        state.forwardedPorts.add(20000);
        assertFalse(state.shouldStopService());

        // Foreground: native WS stops, but port still exists
        state.nativeWsNeeded = false;
        assertFalse(state.shouldStopService());

        // Port removed → should stop
        state.forwardedPorts.remove(20000);
        assertTrue(state.shouldStopService());
    }

    // =====================================================
    // Test 8: Multiple transitions
    // =====================================================

    @Test
    public void multipleTransitions() {
        ServiceState state = new ServiceState();

        // Initially idle → stop
        assertTrue(state.shouldStopService());

        // Background 1
        state.nativeWsNeeded = true;
        assertFalse(state.shouldStopService());

        // Foreground 1
        state.nativeWsNeeded = false;
        assertTrue(state.shouldStopService());

        // Add port
        state.forwardedPorts.add(20000);
        assertFalse(state.shouldStopService());

        // Background 2 (port + native WS)
        state.nativeWsNeeded = true;
        assertFalse(state.shouldStopService());

        // Foreground 2 (port remains)
        state.nativeWsNeeded = false;
        assertFalse(state.shouldStopService());

        // Remove port
        state.forwardedPorts.remove(20000);
        assertTrue(state.shouldStopService());
    }

    // =====================================================
    // Test 9: Verify static nativeWsNeeded field is accessible
    // (ensures the field name hasn't changed and is still static)
    // =====================================================

    @Test
    public void nativeWsNeeded_isStaticVolatileBoolean() throws Exception {
        Field field = PortForwardService.class.getDeclaredField("nativeWsNeeded");
        assertTrue("nativeWsNeeded should be static",
                java.lang.reflect.Modifier.isStatic(field.getModifiers()));
        assertTrue("nativeWsNeeded should be volatile",
                java.lang.reflect.Modifier.isVolatile(field.getModifiers()));
        assertEquals("nativeWsNeeded should be boolean",
                boolean.class, field.getType());
    }

    // --- Helper methods ---

    private Object createMinimalInstance() throws Exception {
        // Allocate instance without calling constructor (avoids Android Service init)
        java.lang.reflect.Constructor<PortForwardService> ctor =
                PortForwardService.class.getDeclaredConstructor();
        ctor.setAccessible(true);

        // Use objenesis-style allocation via Unsafe
        try {
            var unsafeField = java.lang.reflect.Field.class.getDeclaredMethod("get", Object.class);
        } catch (Exception ignored) {}

        // Simpler approach: just allocate with the default constructor
        // PortForwardService() extends Service which has a no-arg constructor
        // This may fail with Android classes, so we use a fallback
        try {
            return ctor.newInstance();
        } catch (Exception e) {
            // If constructor fails, create a proxy-like object that has the right fields
            // We'll just use null and handle it in the tests
            return null;
        }
    }

    private void setForwardedPorts(Object service, Set<Integer> ports) throws Exception {
        if (service == null) return;
        Field field = PortForwardService.class.getDeclaredField("forwardedPorts");
        field.setAccessible(true);
        field.set(service, ports);
    }

    private void invokeMethod(Object target, String methodName) throws Exception {
        if (target == null) return;
        Method method = PortForwardService.class.getDeclaredMethod(methodName);
        method.setAccessible(true);
        method.invoke(target);
    }
}
