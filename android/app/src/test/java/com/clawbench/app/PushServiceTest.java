package com.clawbench.app;

import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Field;
import java.util.HashMap;
import java.util.Map;

import static org.junit.Assert.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Unit tests for PushService's static methods and preference logic.
 *
 * PushService extends android.app.Service, so we test only the static methods
 * that don't require a real Service instance:
 * - setPersistentNotification() writes to SharedPreferences and starts service
 * - isPersistentNotificationEnabled() reads from SharedPreferences
 * - isPersistentNotificationEnabled() defaults to true
 * - PREF_KEY_PERSISTENT_NOTIFICATION is package-visible for testing
 *
 * Note: Build.VERSION.SDK_INT is 0 in unit tests (returnDefaultValues=true),
 * and Intent constructors/setters are no-ops, so we cannot verify Intent
 * action/component values. We verify that startService is called with
 * any(Intent.class), matching the existing test patterns.
 */
public class PushServiceTest {

    private SharedPreferences mockPrefs;
    private Map<String, Object> prefsData;

    @Before
    public void setUp() {
        // Prepare mock SharedPreferences
        prefsData = new HashMap<>();
        mockPrefs = mock(SharedPreferences.class);
        when(mockPrefs.getBoolean(anyString(), anyBoolean())).thenAnswer(inv -> {
            String key = inv.getArgument(0);
            boolean defVal = inv.getArgument(1);
            return prefsData.containsKey(key) ? (Boolean) prefsData.get(key) : defVal;
        });
        when(mockPrefs.edit()).thenAnswer(inv -> {
            SharedPreferences.Editor editor = mock(SharedPreferences.Editor.class);
            when(editor.putBoolean(anyString(), anyBoolean())).thenAnswer(editInv -> {
                String key = editInv.getArgument(0);
                boolean val = editInv.getArgument(1);
                prefsData.put(key, val);
                return editor;
            });
            doNothing().when(editor).apply();
            return editor;
        });
    }

    @After
    public void tearDown() throws Exception {
        // Clean up any static state
        try {
            Field mBinderField = PushService.class.getDeclaredField("mBinder");
            mBinderField.setAccessible(true);
            mBinderField.set(null, null);
        } catch (Exception ignored) {}
    }

    // =====================================================
    // Test 1: isPersistentNotificationEnabled defaults to true
    // =====================================================

    @Test
    public void isPersistentNotificationEnabled_defaultsToTrue() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        // No value set in prefs, should return default (true)
        assertTrue(PushService.isPersistentNotificationEnabled(mockContext));
    }

    // =====================================================
    // Test 2: isPersistentNotificationEnabled reads false
    // =====================================================

    @Test
    public void isPersistentNotificationEnabled_readsFalse() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        prefsData.put(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, false);
        assertFalse(PushService.isPersistentNotificationEnabled(mockContext));
    }

    // =====================================================
    // Test 3: isPersistentNotificationEnabled reads true
    // =====================================================

    @Test
    public void isPersistentNotificationEnabled_readsTrue() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        prefsData.put(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, true);
        assertTrue(PushService.isPersistentNotificationEnabled(mockContext));
    }

    // =====================================================
    // Test 4: setPersistentNotification writes true and starts service
    // =====================================================

    @Test
    public void setPersistentNotification_true_writesAndStartsService() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        PushService.setPersistentNotification(mockContext, true);

        // Verify SharedPreferences was written
        assertTrue((Boolean) prefsData.get(PushService.PREF_KEY_PERSISTENT_NOTIFICATION));

        // Verify that a service was started (startService or startForegroundService
        // depending on SDK_INT — in unit tests, SDK_INT=0 so startService is used)
        verify(mockContext).startService(any(Intent.class));
    }

    // =====================================================
    // Test 5: setPersistentNotification writes false and starts service
    // =====================================================

    @Test
    public void setPersistentNotification_false_writesAndStartsService() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        PushService.setPersistentNotification(mockContext, false);

        // Verify SharedPreferences was written
        assertFalse((Boolean) prefsData.get(PushService.PREF_KEY_PERSISTENT_NOTIFICATION));

        // Verify startService was called (not startForegroundService, since disabling)
        verify(mockContext).startService(any(Intent.class));
    }

    // =====================================================
    // Test 6: PREF_KEY_PERSISTENT_NOTIFICATION value
    // =====================================================

    @Test
    public void prefKey_hasExpectedValue() {
        assertEquals("push_persistent_notification", PushService.PREF_KEY_PERSISTENT_NOTIFICATION);
    }

    // =====================================================
    // Test 7: setPersistentNotification handles exception gracefully
    // =====================================================

    @Test
    public void setPersistentNotification_handlesExceptionGracefully() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);
        // Make startService throw an exception
        when(mockContext.startService(any(Intent.class)))
                .thenThrow(new SecurityException("Not allowed"));

        // Should not throw — exception is caught and logged
        PushService.setPersistentNotification(mockContext, true);

        // Preference should still be written even if service start fails
        assertTrue((Boolean) prefsData.get(PushService.PREF_KEY_PERSISTENT_NOTIFICATION));
    }

    // =====================================================
    // Test 8: isPersistentNotificationEnabled uses correct prefs name
    // =====================================================

    @Test
    public void isPersistentNotificationEnabled_usesCorrectPrefsName() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(anyString(), anyInt())).thenReturn(mockPrefs);

        PushService.isPersistentNotificationEnabled(mockContext);

        verify(mockContext).getSharedPreferences(eq("clawbench_prefs"), eq(Context.MODE_PRIVATE));
    }

    // =====================================================
    // Test 9: setPersistentNotification uses correct prefs name
    // =====================================================

    @Test
    public void setPersistentNotification_usesCorrectPrefsName() {
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(anyString(), anyInt())).thenReturn(mockPrefs);

        PushService.setPersistentNotification(mockContext, false);

        verify(mockContext).getSharedPreferences(eq("clawbench_prefs"), eq(Context.MODE_PRIVATE));
    }

    // =====================================================
    // Test 10: round-trip set then read
    // =====================================================

    @Test
    public void setPersistentNotification_thenRead_roundTrip() {
        Context writeCtx = mock(Context.class);
        when(writeCtx.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);
        when(writeCtx.startService(any(Intent.class))).thenReturn(null);

        Context readCtx = mock(Context.class);
        when(readCtx.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);

        // Set to false
        PushService.setPersistentNotification(writeCtx, false);
        assertFalse(PushService.isPersistentNotificationEnabled(readCtx));

        // Set back to true
        PushService.setPersistentNotification(writeCtx, true);
        assertTrue(PushService.isPersistentNotificationEnabled(readCtx));
    }
}
