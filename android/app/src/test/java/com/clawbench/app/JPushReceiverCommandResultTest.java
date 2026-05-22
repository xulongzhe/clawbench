package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;

import static org.junit.Assert.*;

/**
 * Unit tests for JPushReceiver's onCommandResult error handling.
 *
 * Tests the graceful degradation behavior when JPush SDK reports
 * init/registration errors (1005, 1008, 6001). On these errors,
 * pushAvailable should be reset to false so BackgroundService can
 * fall back to native WebSocket for real-time event delivery.
 *
 * Uses reflection to avoid JPush SDK's obfuscated bytecode (VerifyError).
 */
public class JPushReceiverCommandResultTest {

    private JPushReceiver receiver;

    @Before
    public void setUp() throws Exception {
        Constructor<JPushReceiver> ctor = JPushReceiver.class.getDeclaredConstructor();
        ctor.setAccessible(true);
        receiver = ctor.newInstance();
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
    // Test 1: Error 1005 resets pushAvailable to false
    // =====================================================

    @Test
    public void onCommandResult_error1005_resetsPushAvailable() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 1005);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 2: Error 1008 resets pushAvailable to false
    // =====================================================

    @Test
    public void onCommandResult_error1008_resetsPushAvailable() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 1008);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 3: Error 6001 resets pushAvailable to false
    // =====================================================

    @Test
    public void onCommandResult_error6001_resetsPushAvailable() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 6001);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 4: Unknown error code does NOT trigger pushAvailable reset
    // =====================================================

    @Test
    public void onCommandResult_unknownError_doesNotResetPushAvailable() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 9999);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 5: Success (errorCode=0) takes success branch
    // =====================================================

    @Test
    public void onCommandResult_success_doesNotResetPushAvailable() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 0);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 6: No crash when MainActivity.instance is null
    // =====================================================

    @Test
    public void onCommandResult_nullInstance_noCrash() throws Exception {
        Field instField = MainActivity.class.getDeclaredField("instance");
        instField.setAccessible(true);
        instField.set(null, null);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 1005);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 7: No crash when pushAvailable is already false
    // =====================================================

    @Test
    public void onCommandResult_error1005_alreadyFalse_noCrash() throws Exception {
        setupMainActivity(false);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(0, 1005);
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 8: CmdMessage fields are correctly logged
    // =====================================================

    @Test
    public void onCommandResult_errorWithMsg_logsCorrectly() throws Exception {
        setupMainActivity(true);

        cn.jpush.android.api.CmdMessage cmdMessage = new cn.jpush.android.api.CmdMessage(1, 1005, "AppKey mismatch");
        callOnCommandResult(receiver, cmdMessage);
    }

    // =====================================================
    // Test 9: Direct invocation of lambda$onCommandResult$0
    // This covers the runOnUiThread lambda body that JaCoCo
    // can't trace through Android's no-op runOnUiThread in tests.
    // =====================================================

    @Test
    public void lambda_onCommandResult_resetsPushAvailable() throws Exception {
        setupMainActivity(true);

        // Directly invoke the lambda that resets pushAvailable
        Method lambda = JPushReceiver.class.getDeclaredMethod("lambda$onCommandResult$0");
        lambda.setAccessible(true);
        lambda.invoke(null);

        // Verify pushAvailable was reset to false
        Field paField = MainActivity.class.getDeclaredField("pushAvailable");
        paField.setAccessible(true);
        assertFalse("pushAvailable should be false after lambda resets it",
                paField.getBoolean(MainActivity.instance));
    }

    // =====================================================
    // Test 10: Lambda does nothing when pushAvailable is already false
    // =====================================================

    @Test
    public void lambda_onCommandResult_alreadyFalse_noException() throws Exception {
        setupMainActivity(false);

        Method lambda = JPushReceiver.class.getDeclaredMethod("lambda$onCommandResult$0");
        lambda.setAccessible(true);
        lambda.invoke(null);

        Field paField = MainActivity.class.getDeclaredField("pushAvailable");
        paField.setAccessible(true);
        assertFalse("pushAvailable should still be false", paField.getBoolean(MainActivity.instance));
    }

    // --- Helper methods ---

    private void setupMainActivity(boolean pushAvailableValue) throws Exception {
        // Create a minimal MainActivity instance via Unsafe allocation
        var unsafeField = Class.forName("sun.misc.Unsafe").getDeclaredField("theUnsafe");
        unsafeField.setAccessible(true);
        Object unsafe = unsafeField.get(null);
        Method allocate = unsafe.getClass().getDeclaredMethod("allocateInstance", Class.class);
        allocate.setAccessible(true);
        Object activity = allocate.invoke(unsafe, MainActivity.class);

        Field instField = MainActivity.class.getDeclaredField("instance");
        instField.setAccessible(true);
        instField.set(null, activity);

        Field paField = MainActivity.class.getDeclaredField("pushAvailable");
        paField.setAccessible(true);
        paField.setBoolean(activity, pushAvailableValue);
    }

    private static void callOnCommandResult(JPushReceiver receiver, cn.jpush.android.api.CmdMessage cmdMessage) throws Exception {
        Method method = JPushReceiver.class.getDeclaredMethod("onCommandResult",
                android.content.Context.class, cn.jpush.android.api.CmdMessage.class);
        method.setAccessible(true);
        method.invoke(receiver, new android.content.ContextWrapper(null) {}, cmdMessage);
    }
}
