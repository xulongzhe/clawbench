package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Field;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import static org.junit.Assert.*;

/**
 * Unit tests for BackgroundService.forceReconnect() static method.
 *
 * Tests the control flow of forceReconnect:
 * - Returns false when instance is null
 * - Returns false when ensureConnection fails (no server URL configured)
 * - Resets intentionalDisconnect to false before reconnecting
 *
 * Note: Successful reconnection requires a real SSH server, tested via integration.
 */
public class BackgroundServiceForceReconnectTest {

    private BackgroundService service;
    private ExecutorService testExecutor;

    @Before
    public void setUp() throws Exception {
        // Create a minimal BackgroundService instance via Unsafe allocation
        var unsafeField = Class.forName("sun.misc.Unsafe").getDeclaredField("theUnsafe");
        unsafeField.setAccessible(true);
        Object unsafe = unsafeField.get(null);
        var allocate = unsafe.getClass().getDeclaredMethod("allocateInstance", Class.class);
        allocate.setAccessible(true);
        service = (BackgroundService) allocate.invoke(unsafe, BackgroundService.class);

        // Set static fields
        setStaticField("instance", service);
        setStaticField("isRunning", true);
        setStaticField("lastError", null);

        // Set up networkExecutor (required by forceReconnect)
        testExecutor = Executors.newSingleThreadExecutor();
        Field executorField = BackgroundService.class.getDeclaredField("networkExecutor");
        executorField.setAccessible(true);
        executorField.set(service, testExecutor);

        // Set intentionalDisconnect to false (instance field)
        setInstanceField(service, "intentionalDisconnect", false);
    }

    @After
    public void tearDown() throws Exception {
        try {
            setStaticField("instance", null);
            setStaticField("isRunning", false);
            setStaticField("lastError", null);
        } catch (Exception ignored) {}
        if (testExecutor != null) {
            testExecutor.shutdownNow();
        }
    }

    @Test
    public void testForceReconnect_nullInstance_returnsFalse() throws Exception {
        setStaticField("instance", null);
        boolean result = BackgroundService.forceReconnect(5000);
        assertFalse("Should return false when instance is null", result);
    }

    @Test
    public void testForceReconnect_noServerUrl_returnsFalse() throws Exception {
        // No SharedPreferences with server URL — ensureConnection will fail
        boolean result = BackgroundService.forceReconnect(10000);
        assertFalse("Should return false when ensureConnection fails", result);
    }

    @Test
    public void testForceReconnect_resetsIntentionalDisconnect() throws Exception {
        // Set intentionalDisconnect to true (instance field)
        setInstanceField(service, "intentionalDisconnect", true);

        // Even though ensureConnection will fail, forceReconnect should still reset
        // intentionalDisconnect to false as its first step
        BackgroundService.forceReconnect(10000);

        Field idField = BackgroundService.class.getDeclaredField("intentionalDisconnect");
        idField.setAccessible(true);
        boolean intentionalDisconnect = (boolean) idField.get(service);
        assertFalse("intentionalDisconnect should be reset to false", intentionalDisconnect);
    }

    @Test
    public void testForceReconnect_setsLastErrorOnFailure() throws Exception {
        BackgroundService.forceReconnect(10000);

        // lastError should be set by the failed ensureConnection
        String error = BackgroundService.getLastError();
        assertNotNull("lastError should be set when forceReconnect fails", error);
    }

    // --- Helper methods ---

    private void setStaticField(String name, Object value) throws Exception {
        Field field = BackgroundService.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(null, value);
    }

    private void setInstanceField(Object target, String name, Object value) throws Exception {
        Field field = BackgroundService.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(target, value);
    }
}
