package com.clawbench.app;

import android.content.Intent;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;

import static org.junit.Assert.*;
import static org.mockito.Mockito.*;

/**
 * Unit tests for MainActivity's notification deep linking logic.
 *
 * Tests handleNotificationIntent, getPendingNavigation, handleResumeIntent,
 * redispatchPendingNavigation, and logLaunchIntent — all extracted from
 * lifecycle methods for testability.
 *
 * Uses Unsafe.allocateInstance() to create MainActivity without triggering
 * AppCompatActivity's constructor (which needs Android framework).
 * Uses Mockito to mock Intent for controlling getStringExtra() return values.
 * With returnDefaultValues = true, android.jar stubs are no-ops.
 */
public class MainActivityNotificationTest {

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
    // handleNotificationIntent tests
    // =====================================================

    @Test
    public void handleNotificationIntent_withSessionId_dispatchesEvent() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-123");
        when(mockIntent.getStringExtra("project_path")).thenReturn("/home/user/project");

        // Create a mock WebView that records the JS call
        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // Verify evaluateJavascript was called (the JS event dispatch)
        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-session"),
                org.mockito.ArgumentMatchers.any()
        );

        // Note: pendingNavigation is cleared in the evaluateJavascript callback,
        // which won't fire on a mock WebView. So we can't assert it's null here.
        // Instead, verify intent extras were cleared (happens synchronously)
        verify(mockIntent).removeExtra("session_id");
        verify(mockIntent).removeExtra("project_path");
    }

    @Test
    public void handleNotificationIntent_withSessionIdNoProjectPath() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-456");
        when(mockIntent.getStringExtra("project_path")).thenReturn(null);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-session"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void handleNotificationIntent_nullIntent_returnsEarly() throws Exception {
        // Should not throw
        invokeMethod(activity, "handleNotificationIntent", Intent.class, (Intent) null);
    }

    @Test
    public void handleNotificationIntent_noSessionId_logsOnly() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("session_id")).thenReturn(null);
        when(mockIntent.getStringExtra("project_path")).thenReturn(null);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // Should NOT call evaluateJavascript (no session_id)
        verify(mockWebView, never()).evaluateJavascript(
                org.mockito.ArgumentMatchers.anyString(),
                org.mockito.ArgumentMatchers.any()
        );
    }

    // =====================================================
    // Task notification tests (clawbench-open-task)
    // =====================================================

    @Test
    public void handleNotificationIntent_withTaskId_dispatchesOpenTaskEvent() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("task_id")).thenReturn("2");
        when(mockIntent.getStringExtra("execution_id")).thenReturn("5");
        when(mockIntent.getStringExtra("event_type")).thenReturn("task_update");
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-task");
        when(mockIntent.getStringExtra("project_path")).thenReturn("/home/user/project");

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // Should dispatch clawbench-open-task (not clawbench-open-session)
        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-task"),
                org.mockito.ArgumentMatchers.any()
        );

        // Verify all task-related extras are cleared
        verify(mockIntent).removeExtra("task_id");
        verify(mockIntent).removeExtra("execution_id");
        verify(mockIntent).removeExtra("event_type");
        verify(mockIntent).removeExtra("session_id");
        verify(mockIntent).removeExtra("project_path");
    }

    @Test
    public void handleNotificationIntent_withTaskIdNoExecutionId_dispatchesOpenTask() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("task_id")).thenReturn("3");
        when(mockIntent.getStringExtra("execution_id")).thenReturn(null);
        when(mockIntent.getStringExtra("event_type")).thenReturn("task_update");
        when(mockIntent.getStringExtra("session_id")).thenReturn(null);
        when(mockIntent.getStringExtra("project_path")).thenReturn("/home/user/project");

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-task"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void handleNotificationIntent_withEventTypeTaskUpdateButNoTaskId_fallsBackToSession() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("task_id")).thenReturn(null);
        when(mockIntent.getStringExtra("execution_id")).thenReturn(null);
        when(mockIntent.getStringExtra("event_type")).thenReturn("task_update");
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-fallback");
        when(mockIntent.getStringExtra("project_path")).thenReturn(null);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // Should dispatch clawbench-open-session (no task_id, falls back to session)
        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-session"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void handleNotificationIntent_taskNotification_nullWebView_storesPendingNavigation() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("task_id")).thenReturn("2");
        when(mockIntent.getStringExtra("execution_id")).thenReturn("5");
        when(mockIntent.getStringExtra("event_type")).thenReturn("task_update");
        when(mockIntent.getStringExtra("session_id")).thenReturn(null);
        when(mockIntent.getStringExtra("project_path")).thenReturn("/project");

        setField(activity, "webView", null); // No WebView available

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // pendingNavigation should be set with taskId
        Object pendingNav = getField(activity, "pendingNavigation");
        assertNotNull("pendingNavigation should be stored when webView is null", pendingNav);
        assertTrue("pendingNavigation should contain taskId",
                pendingNav.toString().contains("2"));
    }

    @Test
    public void handleNotificationIntent_nullWebView_storesPendingNavigation() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-789");
        when(mockIntent.getStringExtra("project_path")).thenReturn("/project");

        setField(activity, "webView", null); // No WebView available

        invokeMethod(activity, "handleNotificationIntent", Intent.class, mockIntent);

        // pendingNavigation should be set (not cleared because no webView to dispatch to)
        Object pendingNav = getField(activity, "pendingNavigation");
        assertNotNull("pendingNavigation should be stored when webView is null", pendingNav);
    }

    // =====================================================
    // handleResumeIntent tests
    // =====================================================

    @Test
    public void handleResumeIntent_withNullIntent_callsHandleAndRedispatch() throws Exception {
        // getIntent() returns null with returnDefaultValues=true, so handleNotificationIntent(null) returns early
        // Set up webView so redispatchPendingNavigation has something to check
        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        // Should not throw — covers the method body even with null intent
        invokeMethod(activity, "handleResumeIntent");

        // redispatchPendingNavigation is called but pendingNavigation is null, so no evaluateJavascript
        verify(mockWebView, never()).evaluateJavascript(
                org.mockito.ArgumentMatchers.anyString(),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void handleResumeIntent_withPendingNav_redispatches() throws Exception {
        // Set pendingNavigation so redispatchPendingNavigation will dispatch it
        org.json.JSONObject nav = new org.json.JSONObject();
        nav.put("sessionId", "s-resume");
        setField(activity, "pendingNavigation", nav);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "handleResumeIntent");

        // redispatchPendingNavigation should have dispatched the event
        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-session"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    // =====================================================
    // getPendingNavigation tests
    // =====================================================

    @Test
    public void getPendingNavigation_withData_returnsAndClears() throws Exception {
        // Set pendingNavigation on the activity
        org.json.JSONObject nav = new org.json.JSONObject();
        nav.put("sessionId", "s-test");
        nav.put("projectPath", "/test-project");
        setField(activity, "pendingNavigation", nav);

        // Create WebAppInterface via allocate + set activity field via reflection
        // (avoids constructor issues with Unsafe-allocated MainActivity)
        MainActivity.WebAppInterface bridge = allocate(MainActivity.WebAppInterface.class);
        setField(bridge, "activity", activity);

        String result = bridge.getPendingNavigation();

        assertNotNull("Should return navigation data", result);
        assertTrue("Should contain sessionId", result.contains("s-test"));
        assertTrue("Should contain projectPath", result.contains("/test-project"));

        // Verify pendingNavigation was cleared
        Object pendingNav = getField(activity, "pendingNavigation");
        assertNull("pendingNavigation should be cleared after getPendingNavigation", pendingNav);
    }

    @Test
    public void getPendingNavigation_null_returnsNull() throws Exception {
        setField(activity, "pendingNavigation", null);

        MainActivity.WebAppInterface bridge = allocate(MainActivity.WebAppInterface.class);
        setField(bridge, "activity", activity);

        String result = bridge.getPendingNavigation();

        assertNull("Should return null when no pending navigation", result);
    }

    // =====================================================
    // redispatchPendingNavigation tests
    // =====================================================

    @Test
    public void redispatchPendingNavigation_withPendingNav_dispatches() throws Exception {
        org.json.JSONObject nav = new org.json.JSONObject();
        nav.put("sessionId", "s-redispatch");
        setField(activity, "pendingNavigation", nav);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "redispatchPendingNavigation");

        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-session"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void redispatchPendingNavigation_withTaskNav_dispatchesOpenTask() throws Exception {
        org.json.JSONObject nav = new org.json.JSONObject();
        nav.put("taskId", "2");
        nav.put("executionId", "5");
        setField(activity, "pendingNavigation", nav);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "redispatchPendingNavigation");

        // Should dispatch clawbench-open-task because pendingNavigation has taskId
        verify(mockWebView).evaluateJavascript(
                org.mockito.ArgumentMatchers.contains("clawbench-open-task"),
                org.mockito.ArgumentMatchers.any()
        );
    }

    @Test
    public void redispatchPendingNavigation_noPendingNav_doesNothing() throws Exception {
        setField(activity, "pendingNavigation", null);

        android.webkit.WebView mockWebView = mock(android.webkit.WebView.class);
        setField(activity, "webView", mockWebView);

        invokeMethod(activity, "redispatchPendingNavigation");

        verify(mockWebView, never()).evaluateJavascript(
                org.mockito.ArgumentMatchers.anyString(),
                org.mockito.ArgumentMatchers.any()
        );
    }

    // =====================================================
    // logLaunchIntent tests
    // =====================================================

    @Test
    public void logLaunchIntent_withExtras_logsSessionIdAndProjectPath() throws Exception {
        Intent mockIntent = mock(Intent.class);
        when(mockIntent.getStringExtra("session_id")).thenReturn("s-launch");
        when(mockIntent.getStringExtra("project_path")).thenReturn("/launch-project");

        // Should not throw
        invokeMethod(activity, "logLaunchIntent", Intent.class, mockIntent);
    }

    @Test
    public void logLaunchIntent_nullIntent_doesNothing() throws Exception {
        // Should not throw
        invokeMethod(activity, "logLaunchIntent", Intent.class, (Intent) null);
    }

    // --- Helper methods ---

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

    private static void setField(Object target, String fieldName, Object value) throws Exception {
        Field field = null;
        Class<?> clazz = target.getClass();
        while (clazz != null) {
            try {
                field = clazz.getDeclaredField(fieldName);
                break;
            } catch (NoSuchFieldException e) {
                clazz = clazz.getSuperclass();
            }
        }
        if (field == null) throw new NoSuchFieldException(fieldName);
        field.setAccessible(true);
        field.set(target, value);
    }

    private static Object getField(Object target, String fieldName) throws Exception {
        Field field = null;
        Class<?> clazz = target.getClass();
        while (clazz != null) {
            try {
                field = clazz.getDeclaredField(fieldName);
                break;
            } catch (NoSuchFieldException e) {
                clazz = clazz.getSuperclass();
            }
        }
        if (field == null) throw new NoSuchFieldException(fieldName);
        field.setAccessible(true);
        return field.get(target);
    }

    private static void invokeMethod(Object target, String methodName, Class<?> paramType, Object param) throws Exception {
        Method method = findMethod(target.getClass(), methodName, paramType);
        method.setAccessible(true);
        method.invoke(target, param);
    }

    private static void invokeMethod(Object target, String methodName) throws Exception {
        Method method = findMethod(target.getClass(), methodName);
        method.setAccessible(true);
        method.invoke(target);
    }

    private static Method findMethod(Class<?> clazz, String methodName, Class<?>... paramTypes) throws Exception {
        Class<?> c = clazz;
        while (c != null) {
            try {
                return c.getDeclaredMethod(methodName, paramTypes);
            } catch (NoSuchMethodException e) {
                c = c.getSuperclass();
            }
        }
        throw new NoSuchMethodException(methodName);
    }
}
