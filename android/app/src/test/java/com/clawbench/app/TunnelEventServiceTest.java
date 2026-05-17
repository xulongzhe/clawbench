package com.clawbench.app;

import org.json.JSONObject;
import org.junit.Before;
import org.junit.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.Assert.*;

/**
 * Unit tests for the SSE event handling notification logic extracted from TunnelEventService.
 *
 * Since the actual TunnelEventService extends Android's Service class and depends on
 * framework APIs (NotificationManager, checkSelfPermission, etc.), we test the pure
 * notification decision logic here. The real handleSSEEvent method in TunnelEventService
 * follows the exact same logic.
 *
 * Test coverage:
 * - session_complete: background → notification, foreground → suppressed
 * - task_exec_update: terminal statuses + background → notification
 * - task_exec_update: non-terminal status → no notification
 * - task_exec_update: terminal status + foreground → suppressed
 * - connected event: no notification
 * - unknown event: no crash
 * - malformed JSON: no crash
 * - empty/null event type: no crash
 * - missing fields: sensible defaults
 */
public class TunnelEventServiceTest {

    /** Simulated app foreground state */
    private boolean appInForeground = false;

    /** Captured notification requests from the logic under test */
    private List<NotificationRequest> notifications = new ArrayList<>();

    /** Simple record of a notification that would be shown */
    static class NotificationRequest {
        final String title;
        final String text;
        final String tag;

        NotificationRequest(String title, String text, String tag) {
            this.title = title;
            this.text = text;
            this.tag = tag;
        }
    }

    @Before
    public void setUp() {
        appInForeground = false;
        notifications.clear();
    }

    /**
     * Simulates the notification decision logic from TunnelEventService.handleSSEEvent.
     * This is a pure-Java extraction that mirrors the actual method's switch statement
     * and notification conditions, without any Android framework dependencies.
     */
    private void handleSSEEvent(String eventType, String data) {
        try {
            JSONObject json = new JSONObject(data);

            switch (eventType) {
                case "session_complete": {
                    if (!appInForeground) {
                        String sessionId = json.optString("sessionId", "");
                        String reason = json.optString("reason", "done");
                        String title = "ClawBench";
                        String text = reason.equals("user_cancel") ? "AI response cancelled" : "AI response completed";
                        notifications.add(new NotificationRequest(title, text, "session_" + sessionId));
                    }
                    break;
                }
                case "task_exec_update": {
                    String status = json.optString("status", "");
                    if (("completed".equals(status) || "failed".equals(status) || "cancelled".equals(status))
                            && !appInForeground) {
                        String taskId = json.optString("taskId", "");
                        String title = "ClawBench";
                        String text = "completed".equals(status) ? "Scheduled task completed" :
                                      "failed".equals(status) ? "Scheduled task failed" :
                                      "Scheduled task cancelled";
                        notifications.add(new NotificationRequest(title, text, "task_" + taskId + "_" + status));
                    }
                    break;
                }
                case "connected": {
                    // No notification for connected events
                    break;
                }
            }
        } catch (Exception e) {
            // Graceful handling of malformed data — no crash
        }
    }

    // ───────────────────────────────────────────────────────────────────────
    // session_complete tests
    // ───────────────────────────────────────────────────────────────────────

    @Test
    public void sessionComplete_background_showsNotification() {
        appInForeground = false;

        handleSSEEvent("session_complete", "{\"sessionId\":\"s-1\",\"reason\":\"done\"}");

        assertEquals(1, notifications.size());
        assertEquals("ClawBench", notifications.get(0).title);
        assertEquals("AI response completed", notifications.get(0).text);
        assertEquals("session_s-1", notifications.get(0).tag);
    }

    @Test
    public void sessionComplete_foreground_noNotification() {
        appInForeground = true;

        handleSSEEvent("session_complete", "{\"sessionId\":\"s-1\",\"reason\":\"done\"}");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void sessionComplete_userCancel_showsCancelledText() {
        appInForeground = false;

        handleSSEEvent("session_complete", "{\"sessionId\":\"s-2\",\"reason\":\"user_cancel\"}");

        assertEquals(1, notifications.size());
        assertEquals("AI response cancelled", notifications.get(0).text);
    }

    @Test
    public void sessionComplete_missingReason_defaultsToDone() {
        appInForeground = false;

        handleSSEEvent("session_complete", "{\"sessionId\":\"s-3\"}");

        assertEquals(1, notifications.size());
        // Missing reason defaults to "done" via optString("reason", "done")
        // "done" is not "user_cancel", so text = "AI response completed"
        assertEquals("AI response completed", notifications.get(0).text);
    }

    // ───────────────────────────────────────────────────────────────────────
    // task_exec_update tests
    // ───────────────────────────────────────────────────────────────────────

    @Test
    public void taskExecUpdate_completed_background_showsNotification() {
        appInForeground = false;

        handleSSEEvent("task_exec_update", "{\"taskId\":42,\"execId\":\"e-1\",\"status\":\"completed\"}");

        assertEquals(1, notifications.size());
        assertEquals("Scheduled task completed", notifications.get(0).text);
        assertEquals("task_42_completed", notifications.get(0).tag);
    }

    @Test
    public void taskExecUpdate_failed_background_showsFailedText() {
        appInForeground = false;

        handleSSEEvent("task_exec_update", "{\"taskId\":43,\"status\":\"failed\"}");

        assertEquals(1, notifications.size());
        assertEquals("Scheduled task failed", notifications.get(0).text);
    }

    @Test
    public void taskExecUpdate_cancelled_background_showsCancelledText() {
        appInForeground = false;

        handleSSEEvent("task_exec_update", "{\"taskId\":44,\"status\":\"cancelled\"}");

        assertEquals(1, notifications.size());
        assertEquals("Scheduled task cancelled", notifications.get(0).text);
    }

    @Test
    public void taskExecUpdate_running_background_noNotification() {
        appInForeground = false;

        handleSSEEvent("task_exec_update", "{\"taskId\":45,\"status\":\"running\"}");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void taskExecUpdate_completed_foreground_noNotification() {
        appInForeground = true;

        handleSSEEvent("task_exec_update", "{\"taskId\":46,\"status\":\"completed\"}");

        assertTrue(notifications.isEmpty());
    }

    // ───────────────────────────────────────────────────────────────────────
    // Other event types and edge cases
    // ───────────────────────────────────────────────────────────────────────

    @Test
    public void connectedEvent_noNotification() {
        handleSSEEvent("connected", "{\"clientId\":\"test-123\"}");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void unknownEventType_noCrash() {
        handleSSEEvent("unknown_event", "{\"foo\":\"bar\"}");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void malformedJson_noCrash() {
        handleSSEEvent("session_complete", "not valid json {{{");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void emptyEventType_noCrash() {
        handleSSEEvent("", "{\"status\":\"completed\"}");

        assertTrue(notifications.isEmpty());
    }

    @Test
    public void nullSafeHandling_emptyData() {
        handleSSEEvent("session_complete", "{}");

        // Should not crash — sessionId defaults to "", reason defaults to "done"
        assertEquals(1, notifications.size());
    }
}
