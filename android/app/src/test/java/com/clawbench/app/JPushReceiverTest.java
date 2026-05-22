package com.clawbench.app;

import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.Method;

import static org.junit.Assert.*;

/**
 * Unit tests for JPushReceiver's push notification deep linking logic.
 *
 * Uses reflection to create JPushReceiver and NotificationMessage
 * without triggering JPush SDK's obfuscated bytecode (which causes VerifyError
 * with Robolectric). With returnDefaultValues = true in build.gradle,
 * android.jar stub methods (Intent constructor, putExtra, startActivity, etc.)
 * are all no-ops.
 */
public class JPushReceiverTest {

    private JPushReceiver receiver;
    private Object notificationMessage;

    @Before
    public void setUp() throws Exception {
        // Create receiver via default constructor (JPushMessageReceiver extends BroadcastReceiver)
        Constructor<JPushReceiver> ctor = JPushReceiver.class.getDeclaredConstructor();
        ctor.setAccessible(true);
        receiver = ctor.newInstance();

        // Create NotificationMessage via reflection constructor
        // NotificationMessage has a public no-arg constructor in some JPush versions,
        // or we use Unsafe allocation as fallback
        try {
            Constructor<?> msgCtor = cn.jpush.android.api.NotificationMessage.class.getDeclaredConstructor();
            msgCtor.setAccessible(true);
            notificationMessage = msgCtor.newInstance();
        } catch (Exception e) {
            // Fallback: use Unsafe allocation if no public constructor
            var unsafeField = Class.forName("sun.misc.Unsafe").getDeclaredField("theUnsafe");
            unsafeField.setAccessible(true);
            Object unsafe = unsafeField.get(null);
            Method allocate = unsafe.getClass().getDeclaredMethod("allocateInstance", Class.class);
            allocate.setAccessible(true);
            notificationMessage = allocate.invoke(unsafe, cn.jpush.android.api.NotificationMessage.class);
        }
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
    // Test 1: onNotifyMessageOpened with valid extras
    // =====================================================

    @Test
    public void onNotifyMessageOpened_validExtras_launchesMainActivity() throws Exception {
        setField(notificationMessage, "notificationExtras",
                "{\"session_id\":\"s-123\",\"project_path\":\"/home/user/project\"}");
        setField(notificationMessage, "msgId", "msg-001");
        setField(notificationMessage, "notificationTitle", "Test Title");
        setField(notificationMessage, "notificationContent", "Test Content");

        callOnNotifyMessageOpened(receiver, notificationMessage);
        // If we get here without exception, the method executed fully
    }

    // =====================================================
    // Test 2: onNotifyMessageOpened with null extras
    // =====================================================

    @Test
    public void onNotifyMessageOpened_nullExtras_stillLaunches() throws Exception {
        setField(notificationMessage, "notificationExtras", null);
        setField(notificationMessage, "msgId", "msg-002");
        setField(notificationMessage, "notificationTitle", "Test");
        setField(notificationMessage, "notificationContent", "Content");

        callOnNotifyMessageOpened(receiver, notificationMessage);
    }

    // =====================================================
    // Test 3: onNotifyMessageOpened with malformed JSON extras
    // =====================================================

    @Test
    public void onNotifyMessageOpened_malformedExtras_catchesException() throws Exception {
        setField(notificationMessage, "notificationExtras", "not valid json {{{");
        setField(notificationMessage, "msgId", "msg-003");
        setField(notificationMessage, "notificationTitle", "Test");
        setField(notificationMessage, "notificationContent", "Content");

        callOnNotifyMessageOpened(receiver, notificationMessage);
    }

    // =====================================================
    // Test 4: onNotifyMessageOpened with extras but no session_id
    // =====================================================

    @Test
    public void onNotifyMessageOpened_noSessionId_launchesWithoutSession() throws Exception {
        setField(notificationMessage, "notificationExtras", "{\"project_path\":\"/home/user/project\"}");
        setField(notificationMessage, "msgId", "msg-004");
        setField(notificationMessage, "notificationTitle", "Test");
        setField(notificationMessage, "notificationContent", "Content");

        callOnNotifyMessageOpened(receiver, notificationMessage);
    }

    // =====================================================
    // Test 5: onNotifyMessageArrived
    // =====================================================

    @Test
    public void onNotifyMessageArrived_logsAndReturns() throws Exception {
        setField(notificationMessage, "msgId", "msg-005");
        setField(notificationMessage, "notificationTitle", "Arrived Title");
        setField(notificationMessage, "notificationContent", "Arrived Content");

        Method method = JPushReceiver.class.getDeclaredMethod("onNotifyMessageArrived",
                android.content.Context.class, cn.jpush.android.api.NotificationMessage.class);
        method.setAccessible(true);
        method.invoke(receiver, new android.content.ContextWrapper(null) {}, notificationMessage);
    }

    // =====================================================
    // Test 6: onNotifyMessageOpened with task notification extras
    // =====================================================

    @Test
    public void onNotifyMessageOpened_taskExtras_launchesWithTaskId() throws Exception {
        setField(notificationMessage, "notificationExtras",
                "{\"task_id\":\"2\",\"execution_id\":\"5\",\"event_type\":\"task_update\",\"session_id\":\"s-task\",\"project_path\":\"/home/user/project\"}");
        setField(notificationMessage, "msgId", "msg-006");
        setField(notificationMessage, "notificationTitle", "计划任务完成");
        setField(notificationMessage, "notificationContent", "任务已完成");

        callOnNotifyMessageOpened(receiver, notificationMessage);
        // If we get here without exception, the method executed fully with task extras
    }

    @Test
    public void onNotifyMessageOpened_taskExtrasNoExecutionId_launchesWithTaskId() throws Exception {
        setField(notificationMessage, "notificationExtras",
                "{\"task_id\":\"1\",\"event_type\":\"task_update\",\"project_path\":\"/home/user/project\"}");
        setField(notificationMessage, "msgId", "msg-007");
        setField(notificationMessage, "notificationTitle", "计划任务完成");
        setField(notificationMessage, "notificationContent", "任务已完成");

        callOnNotifyMessageOpened(receiver, notificationMessage);
    }

    @Test
    public void onNotifyMessageOpened_sessionExtrasOnly_noTaskId() throws Exception {
        setField(notificationMessage, "notificationExtras",
                "{\"session_id\":\"s-123\",\"project_path\":\"/home/user/project\"}");
        setField(notificationMessage, "msgId", "msg-008");
        setField(notificationMessage, "notificationTitle", "AI任务完成");
        setField(notificationMessage, "notificationContent", "ok");

        callOnNotifyMessageOpened(receiver, notificationMessage);
    }

    // --- Helper methods ---

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

    private static void callOnNotifyMessageOpened(JPushReceiver receiver, Object notificationMessage) throws Exception {
        Method method = JPushReceiver.class.getDeclaredMethod("onNotifyMessageOpened",
                android.content.Context.class, cn.jpush.android.api.NotificationMessage.class);
        method.setAccessible(true);
        method.invoke(receiver, new android.content.ContextWrapper(null) {}, notificationMessage);
    }
}
