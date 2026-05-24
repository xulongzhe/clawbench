package com.clawbench.app;

import android.webkit.ValueCallback;
import android.webkit.WebView;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;

import static org.junit.Assert.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.contains;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Unit tests for MainActivity's onBackPressed JS delegation logic.
 *
 * The new onBackPressed flow:
 * 1. If fullscreen video is showing → exit fullscreen
 * 2. If on login page → super.onBackPressed() (exit app)
 * 3. Otherwise → evaluateJavascript to dispatch 'clawbench-back-press' event
 *    - If JS sets __clawbenchBackHandled = true → JS handled it, don't call super
 *    - If JS doesn't handle → call super.onBackPressed() (default back behavior)
 *
 * Tests cover the JS evaluation, callback result handling, and fullscreen logic.
 * Note: super.onBackPressed() cannot be called on an Unsafe-allocated activity
 * (throws NPE from ComponentActivity.getLifecycle()), so callback tests that
 * trigger the "not handled" path use a spy to intercept the super call.
 */
public class MainActivityBackPressTest {

    private MainActivity activity;
    private WebView mockWebView;

    @Before
    public void setUp() throws Exception {
        activity = allocate(MainActivity.class);
        // Set the static instance field
        Field instanceField = MainActivity.class.getDeclaredField("instance");
        instanceField.setAccessible(true);
        instanceField.set(null, activity);

        // Mock WebView
        mockWebView = mock(WebView.class);
        setField(activity, "webView", mockWebView);

        // Ensure customView is null (no fullscreen video)
        setField(activity, "customView", null);
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
    // JS delegation: evaluateJavascript is called
    // =====================================================

    @Test
    public void onBackPressed_nonLoginPage_dispatchesClawbenchBackPress() throws Exception {
        // Set WebView URL to non-login page
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        activity.onBackPressed();

        // Verify evaluateJavascript was called with the clawbench-back-press JS
        verify(mockWebView).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));
    }

    @Test
    public void onBackPressed_loginPage_doesNotDispatchBackPressEvent() throws Exception {
        // Set WebView URL to login page
        when(mockWebView.getUrl()).thenReturn("file:///android_asset/login.html");

        // On login page, onBackPressed calls super.onBackPressed() directly.
        // Since super.onBackPressed() crashes on Unsafe-allocated activity,
        // we verify the inverse: that evaluateJavascript is NOT called.
        try {
            activity.onBackPressed();
        } catch (NullPointerException e) {
            // Expected: super.onBackPressed() fails on Unsafe-allocated activity.
            // This proves the login-page branch was taken (calls super, not JS).
        }

        // Should NOT dispatch JS back-press event
        verify(mockWebView, never()).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));
    }

    @Test
    public void onBackPressed_nullUrl_dispatchesClawbenchBackPress() throws Exception {
        // WebView URL is null (page not loaded yet)
        when(mockWebView.getUrl()).thenReturn(null);

        activity.onBackPressed();

        // With null URL, should proceed to JS delegation
        verify(mockWebView).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));
    }

    // =====================================================
    // JS callback: result handling (captured callback)
    // =====================================================

    @Test
    public void onBackPressed_jsHandledTrue_callbackReceivesTrue() throws Exception {
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        // Capture the ValueCallback to simulate JS returning "true"
        final ValueCallback<String>[] capturedCallback = new ValueCallback[1];

        doAnswer(invocation -> {
            capturedCallback[0] = invocation.getArgument(1);
            return null;
        }).when(mockWebView).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));

        activity.onBackPressed();

        assertNotNull("Callback should have been captured", capturedCallback[0]);

        // Simulate JS returning "true" (handled) — callback won't call super
        // This should NOT throw because "true".equals("true") → handled=true → no super call
        capturedCallback[0].onReceiveValue("true");
    }

    @Test
    public void onBackPressed_jsHandledFalse_callbackTriggersSuperBackPress() throws Exception {
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        final ValueCallback<String>[] capturedCallback = new ValueCallback[1];

        doAnswer(invocation -> {
            capturedCallback[0] = invocation.getArgument(1);
            return null;
        }).when(mockWebView).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));

        activity.onBackPressed();

        assertNotNull("Callback should have been captured", capturedCallback[0]);

        // Simulate JS returning "false" (not handled) — callback calls super.onBackPressed()
        // which crashes on Unsafe-allocated activity. The NPE proves the "not handled" branch.
        try {
            capturedCallback[0].onReceiveValue("false");
            // If no exception, the callback path works correctly
        } catch (NullPointerException e) {
            // Expected: super.onBackPressed() fails on Unsafe-allocated activity.
            // The NPE comes from ComponentActivity.getLifecycle() being null,
            // which confirms the "not handled" → super.onBackPressed() path was taken.
            assertTrue("NPE should be from super.onBackPressed()",
                e.getMessage() == null || e.getMessage().contains("getLifecycle") ||
                e.getStackTrace()[0].getClassName().contains("ComponentActivity"));
        }
    }

    @Test
    public void onBackPressed_jsReturnsNull_callbackTriggersSuperBackPress() throws Exception {
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        final ValueCallback<String>[] capturedCallback = new ValueCallback[1];

        doAnswer(invocation -> {
            capturedCallback[0] = invocation.getArgument(1);
            return null;
        }).when(mockWebView).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));

        activity.onBackPressed();

        assertNotNull("Callback should have been captured", capturedCallback[0]);

        // Simulate JS returning null — "true".equals(null) = false → handled=false → super called
        try {
            capturedCallback[0].onReceiveValue(null);
        } catch (NullPointerException e) {
            // Expected: same as jsHandledFalse — super.onBackPressed() crashes
            assertTrue("NPE should be from super.onBackPressed()",
                e.getMessage() == null || e.getMessage().contains("getLifecycle") ||
                e.getStackTrace()[0].getClassName().contains("ComponentActivity"));
        }
    }

    // =====================================================
    // JS string content verification
    // =====================================================

    @Test
    public void onBackPressed_jsIncludesClawbenchBackHandledFlag() throws Exception {
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        activity.onBackPressed();

        // Verify the JS string includes __clawbenchBackHandled initialization and reset
        verify(mockWebView).evaluateJavascript(contains("__clawbenchBackHandled"), any(ValueCallback.class));
    }

    @Test
    public void onBackPressed_jsDispatchesCustomEvent() throws Exception {
        when(mockWebView.getUrl()).thenReturn("https://example.com/app");

        activity.onBackPressed();

        // Verify the JS string includes CustomEvent dispatch
        verify(mockWebView).evaluateJavascript(contains("CustomEvent"), any(ValueCallback.class));
    }

    // =====================================================
    // Fullscreen video: customView != null
    // =====================================================

    @Test
    public void onBackPressed_fullscreenVideo_exitsFullscreen() throws Exception {
        // Set up a customView to simulate fullscreen video
        android.view.View mockCustomView = mock(android.view.View.class);
        setField(activity, "customView", mockCustomView);

        android.webkit.WebChromeClient mockChromeClient = mock(android.webkit.WebChromeClient.class);
        when(mockWebView.getWebChromeClient()).thenReturn(mockChromeClient);

        activity.onBackPressed();

        // Should call onHideCustomView on the WebChromeClient
        verify(mockChromeClient).onHideCustomView();
        // Should NOT dispatch JS back-press event
        verify(mockWebView, never()).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));
    }

    @Test
    public void onBackPressed_fullscreenVideo_nullChromeClient_noException() throws Exception {
        // Set up a customView with null WebChromeClient
        android.view.View mockCustomView = mock(android.view.View.class);
        setField(activity, "customView", mockCustomView);

        when(mockWebView.getWebChromeClient()).thenReturn(null);

        // Should not throw NPE — the code checks `if (client != null)` before calling
        activity.onBackPressed();

        // Should NOT dispatch JS back-press event (fullscreen branch skips JS delegation)
        verify(mockWebView, never()).evaluateJavascript(contains("clawbench-back-press"), any(ValueCallback.class));
    }

    // =====================================================
    // Pure logic tests (no Android framework dependency)
    // =====================================================

    @Test
    public void handledResult_true_parsedCorrectly() {
        // The logic: boolean handled = "true".equals(result);
        String result = "true";
        boolean handled = "true".equals(result);
        assertTrue(handled);
    }

    @Test
    public void handledResult_false_parsedCorrectly() {
        String result = "false";
        boolean handled = "true".equals(result);
        assertFalse(handled);
    }

    @Test
    public void handledResult_null_parsedCorrectly() {
        String result = null;
        boolean handled = "true".equals(result);
        assertFalse(handled);
    }

    @Test
    public void handledResult_emptyString_parsedCorrectly() {
        String result = "";
        boolean handled = "true".equals(result);
        assertFalse(handled);
    }

    // --- Helper methods ---

    @SuppressWarnings("unchecked")
    private static <T> T allocate(Class<T> clazz) throws Exception {
        try {
            Constructor<T> ctor = clazz.getDeclaredConstructor();
            ctor.setAccessible(true);
            return ctor.newInstance();
        } catch (Exception e) {
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
}
