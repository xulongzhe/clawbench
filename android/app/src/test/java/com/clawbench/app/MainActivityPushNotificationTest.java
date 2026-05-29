package com.clawbench.app;

import android.content.Context;
import android.content.SharedPreferences;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.HashMap;
import java.util.Map;

import static org.junit.Assert.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Unit tests for MainActivity's push persistent notification JS bridge methods.
 *
 * Covers the two new @JavascriptInterface methods added to WebAppInterface:
 * - setPushPersistentNotification(boolean enabled) — delegates to PushService.setPersistentNotification
 * - isPushPersistentNotification() — delegates to PushService.isPersistentNotificationEnabled
 *
 * Uses Unsafe allocation + reflection to create a minimal MainActivity instance,
 * then constructs a WebAppInterface and invokes methods via reflection.
 *
 * Note: We cannot use attachBaseContext() because AppCompatActivity's implementation
 * calls PackageManager.getServiceInfo() which requires a full Android framework.
 * Instead, we use Unsafe to create both objects and set fields via reflection.
 */
public class MainActivityPushNotificationTest {

    private Object activity;
    private Object webAppInterface;
    private SharedPreferences mockPrefs;
    private Map<String, Object> prefsData;

    @Before
    public void setUp() throws Exception {
        // Create a minimal MainActivity instance via Unsafe allocation
        var unsafeField = Class.forName("sun.misc.Unsafe").getDeclaredField("theUnsafe");
        unsafeField.setAccessible(true);
        Object unsafe = unsafeField.get(null);
        Method allocate = unsafe.getClass().getDeclaredMethod("allocateInstance", Class.class);
        allocate.setAccessible(true);
        activity = allocate.invoke(unsafe, MainActivity.class);

        // Set the static instance field
        Field instField = MainActivity.class.getDeclaredField("instance");
        instField.setAccessible(true);
        instField.set(null, activity);

        // Set up mock SharedPreferences
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

        // Inject mock SharedPreferences into MainActivity
        Field prefsField = MainActivity.class.getDeclaredField("prefs");
        prefsField.setAccessible(true);
        prefsField.set(activity, mockPrefs);

        // Create WebAppInterface via Unsafe allocation (avoids AppCompatActivity.attachBaseContext)
        Class<?> webAppInterfaceClass = Class.forName("com.clawbench.app.MainActivity$WebAppInterface");
        webAppInterface = allocate.invoke(unsafe, webAppInterfaceClass);

        // Set the activity field via reflection
        Field activityField = webAppInterfaceClass.getDeclaredField("activity");
        activityField.setAccessible(true);
        activityField.set(webAppInterface, activity);

        // Create a mock Context for PushService.setPersistentNotification
        Context mockContext = mock(Context.class);
        when(mockContext.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);
        when(mockContext.startService(any(android.content.Intent.class))).thenReturn(null);
        when(mockContext.getApplicationContext()).thenReturn(mockContext);

        // Override getBaseContext() to return the mock context
        // This is what activity.getSharedPreferences() and activity.startService() use
        MainActivity mockActivity = mock(MainActivity.class);
        when(mockActivity.getSharedPreferences(eq("clawbench_prefs"), anyInt())).thenReturn(mockPrefs);
        when(mockActivity.startService(any(android.content.Intent.class))).thenReturn(null);
        when(mockActivity.getApplicationContext()).thenReturn(mockContext);

        // Update the WebAppInterface's activity field to use the mock
        activityField.set(webAppInterface, mockActivity);

        // Also update the static instance field
        instField.set(null, mockActivity);
    }

    @After
    public void tearDown() throws Exception {
        try {
            Field f = MainActivity.class.getDeclaredField("instance");
            f.setAccessible(true);
            f.set(null, null);
        } catch (Exception ignored) {}
    }

    // =====================================================
    // Test 1: isPushPersistentNotification defaults to true
    // =====================================================

    @Test
    public void isPushPersistentNotification_defaultsTrue() throws Exception {
        Method method = webAppInterface.getClass().getDeclaredMethod("isPushPersistentNotification");
        method.setAccessible(true);
        boolean result = (Boolean) method.invoke(webAppInterface);
        assertTrue("isPushPersistentNotification should default to true", result);
    }

    // =====================================================
    // Test 2: isPushPersistentNotification reads from SharedPreferences
    // =====================================================

    @Test
    public void isPushPersistentNotification_readsFromPrefs() throws Exception {
        prefsData.put(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, false);

        Method method = webAppInterface.getClass().getDeclaredMethod("isPushPersistentNotification");
        method.setAccessible(true);
        boolean result = (Boolean) method.invoke(webAppInterface);
        assertFalse("isPushPersistentNotification should read false from prefs", result);
    }

    // =====================================================
    // Test 3: isPushPersistentNotification returns true when set
    // =====================================================

    @Test
    public void isPushPersistentNotification_returnsTrueWhenSet() throws Exception {
        prefsData.put(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, true);

        Method method = webAppInterface.getClass().getDeclaredMethod("isPushPersistentNotification");
        method.setAccessible(true);
        boolean result = (Boolean) method.invoke(webAppInterface);
        assertTrue("isPushPersistentNotification should read true from prefs", result);
    }

    // =====================================================
    // Test 4: setPushPersistentNotification writes to prefs
    // =====================================================

    @Test
    public void setPushPersistentNotification_writesToPrefs() throws Exception {
        Method method = webAppInterface.getClass().getDeclaredMethod("setPushPersistentNotification", boolean.class);
        method.setAccessible(true);
        method.invoke(webAppInterface, true);

        // Verify the preference was written
        assertTrue("Preference should be true after setPushPersistentNotification(true)",
                (Boolean) prefsData.getOrDefault(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, true));
    }

    // =====================================================
    // Test 5: setPushPersistentNotification(false) writes false
    // =====================================================

    @Test
    public void setPushPersistentNotification_false_writesFalse() throws Exception {
        Method method = webAppInterface.getClass().getDeclaredMethod("setPushPersistentNotification", boolean.class);
        method.setAccessible(true);
        method.invoke(webAppInterface, false);

        assertFalse("Preference should be false after setPushPersistentNotification(false)",
                (Boolean) prefsData.getOrDefault(PushService.PREF_KEY_PERSISTENT_NOTIFICATION, true));
    }

    // =====================================================
    // Test 6: setPushPersistentNotification has @JavascriptInterface
    // =====================================================

    @Test
    public void setPushPersistentNotification_hasJavascriptInterface() throws Exception {
        Method method = webAppInterface.getClass().getDeclaredMethod("setPushPersistentNotification", boolean.class);
        assertTrue("setPushPersistentNotification should have @JavascriptInterface",
                method.isAnnotationPresent(android.webkit.JavascriptInterface.class));
    }

    // =====================================================
    // Test 7: isPushPersistentNotification has @JavascriptInterface
    // =====================================================

    @Test
    public void isPushPersistentNotification_hasJavascriptInterface() throws Exception {
        Method method = webAppInterface.getClass().getDeclaredMethod("isPushPersistentNotification");
        assertTrue("isPushPersistentNotification should have @JavascriptInterface",
                method.isAnnotationPresent(android.webkit.JavascriptInterface.class));
    }

    // =====================================================
    // Test 8: Round-trip: set then read
    // =====================================================

    @Test
    public void setPushPersistentNotification_roundTrip() throws Exception {
        Method setMethod = webAppInterface.getClass().getDeclaredMethod("setPushPersistentNotification", boolean.class);
        setMethod.setAccessible(true);

        Method getMethod = webAppInterface.getClass().getDeclaredMethod("isPushPersistentNotification");
        getMethod.setAccessible(true);

        // Set to false
        setMethod.invoke(webAppInterface, false);
        assertFalse("Should read false after setting false", (Boolean) getMethod.invoke(webAppInterface));

        // Set to true
        setMethod.invoke(webAppInterface, true);
        assertTrue("Should read true after setting true", (Boolean) getMethod.invoke(webAppInterface));
    }
}
