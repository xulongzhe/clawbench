package com.clawbench.app;

import android.content.Context;
import android.content.Intent;
import cn.jpush.android.api.CmdMessage;
import cn.jpush.android.api.NotificationMessage;
import cn.jpush.android.service.JPushMessageReceiver;

public class JPushReceiver extends JPushMessageReceiver {
    private static final String TAG = "ClawBench";

    @Override
    public void onNotifyMessageArrived(Context context, NotificationMessage message) {
        AppLog.i(TAG, "JPush: notification arrived, msgId=" + message.msgId
                + ", title=" + message.notificationTitle
                + ", content=" + message.notificationContent);
    }

    @Override
    public void onNotifyMessageOpened(Context context, NotificationMessage message) {
        AppLog.i(TAG, "JPush: notification OPENED, msgId=" + message.msgId
                + ", title=" + message.notificationTitle
                + ", content=" + message.notificationContent);
        AppLog.i(TAG, "JPush: notificationExtras=" + message.notificationExtras);

        // Extract session_id and project_path from notification extras
        String sessionId = null;
        String projectPath = null;
        if (message.notificationExtras != null) {
            try {
                org.json.JSONObject extras = new org.json.JSONObject(message.notificationExtras);
                AppLog.i(TAG, "JPush: parsed extras JSON: " + extras.toString());
                sessionId = extras.optString("session_id", null);
                projectPath = extras.optString("project_path", null);
                AppLog.i(TAG, "JPush: extracted sessionId=" + sessionId + ", projectPath=" + projectPath);
            } catch (Exception e) {
                AppLog.w(TAG, "JPush: failed to parse notification extras", e);
            }
        } else {
            AppLog.w(TAG, "JPush: notificationExtras is null, cannot extract session_id");
        }

        // Build safe JSON parameter to prevent XSS injection
        final String jsArg;
        try {
            org.json.JSONObject detail = new org.json.JSONObject();
            if (sessionId != null) detail.put("sessionId", sessionId);
            if (projectPath != null) detail.put("projectPath", projectPath);
            jsArg = detail.toString();
            AppLog.i(TAG, "JPush: built navigation jsArg=" + jsArg);
        } catch (Exception e) {
            AppLog.w(TAG, "JPush: failed to build JSON for open-session event", e);
            return;
        }

        // CRITICAL: Bring the app to the foreground by launching MainActivity.
        // JPush SDK's JNotifyActivity is a transparent activity that calls
        // onNotifyMessageOpened then finishes itself. It does NOT bring our
        // MainActivity to the foreground — we must do that explicitly.
        // Using FLAG_ACTIVITY_NEW_TASK | FLAG_ACTIVITY_SINGLE_TOP | FLAG_ACTIVITY_CLEAR_TOP
        // ensures: NEW_TASK (required from non-Activity context),
        //          SINGLE_TOP (reuse existing MainActivity if alive, triggers onNewIntent),
        //          CLEAR_TOP (clear any activities on top of MainActivity in the task).
        AppLog.i(TAG, "JPush: launching MainActivity to bring app to foreground");
        Intent launchIntent = new Intent(context, MainActivity.class);
        launchIntent.setAction(Intent.ACTION_MAIN);
        launchIntent.addCategory(Intent.CATEGORY_LAUNCHER);
        launchIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK
                | Intent.FLAG_ACTIVITY_SINGLE_TOP
                | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        // Pass navigation data as intent extras
        if (sessionId != null) launchIntent.putExtra("session_id", sessionId);
        if (projectPath != null) launchIntent.putExtra("project_path", projectPath);
        context.startActivity(launchIntent);
        AppLog.i(TAG, "JPush: startActivity called with navigation extras");
    }

    @Override
    public void onRegister(Context context, String registrationId) {
        AppLog.i(TAG, "JPush registered: " + registrationId);
        // Notify the WebView layer so it can register the push ID via WS.
        // Push registration is now done via WS "register" message (tied to the
        // WS session), so we don't need a separate HTTP endpoint anymore.
        notifyWebView(registrationId);
    }

    @Override
    public void onConnected(Context context, boolean isConnected) {
        AppLog.i(TAG, "JPush connected: " + isConnected);
    }

    /**
     * Called when JPush SDK reports a command result, including errors.
     * Error codes: 1005 = AppKey/package name mismatch, 1008 = AppKey not registered.
     * These typically happen when the server-provided AppKey doesn't match the
     * package name registered on the JPush dashboard, or in debug builds where
     * the applicationId has a ".debug" suffix.
     *
     * On error, we reset pushAvailable so BackgroundService falls back to native WS,
     * and log the error for diagnostics. The app degrades gracefully — push is
     * unavailable but real-time events still arrive via WebSocket.
     */
    @Override
    public void onCommandResult(Context context, CmdMessage cmdMessage) {
        int errorCode = cmdMessage.errorCode;
        if (errorCode != 0) {
            AppLog.w(TAG, "JPush: command error, cmd=" + cmdMessage.cmd
                    + ", code=" + errorCode
                    + ", msg=" + cmdMessage.msg);

            // Known init/registration errors that mean push won't work:
            // 1005 = AppKey and package name don't match
            // 1008 = AppKey is not registered on JPush dashboard
            // 6001 = invalid AppKey format
            if (errorCode == 1005 || errorCode == 1008 || errorCode == 6001) {
                AppLog.w(TAG, "JPush: init/registration failed (code=" + errorCode
                        + "), push unavailable — falling back to WebSocket");
                // Reset both pushAvailable and jpushEnabledOnServer on the main thread
                // so BackgroundService can restore native WS for real-time event delivery.
                // jpushEnabledOnServer must also be reset because BackgroundService.onMessage
                // checks (pushAvailable || jpushEnabledOnServer) — if jpushEnabledOnServer
                // stays true, native WS won't be reconnected even though push is broken.
                if (MainActivity.instance != null) {
                    MainActivity.instance.runOnUiThread(() -> {
                        if (MainActivity.instance.pushAvailable) {
                            MainActivity.instance.pushAvailable = false;
                        }
                        if (MainActivity.instance.jpushEnabledOnServer) {
                            MainActivity.instance.jpushEnabledOnServer = false;
                        }
                        AppLog.i(TAG, "JPush: pushAvailable and jpushEnabledOnServer reset to false, native WS will resume");
                    });
                }
                // Proactively restart native WS to ensure background event delivery.
                // When the app is in the background, onPause() already ran and skipped
                // starting native WS because jpushEnabledOnServer was true. Now that
                // push has failed, we must start native WS immediately, otherwise the
                // user gets no notifications until they bring the app to the foreground.
                BackgroundService.startNativeEventWs(context);
            }
        } else {
            AppLog.d(TAG, "JPush: command success, cmd=" + cmdMessage.cmd);
        }
    }

    /**
     * Notify the WebView layer that the JPush Registration ID is now available.
     * The WebView's useGlobalEvents will receive this event and send a WS
     * "register" message to the server with the registration ID.
     */
    private void notifyWebView(String registrationId) {
        if (MainActivity.instance == null) return;
        // Build safe JSON parameter to prevent XSS injection via registrationId
        final String jsArg;
        try {
            org.json.JSONObject detail = new org.json.JSONObject();
            detail.put("registrationId", registrationId);
            jsArg = detail.toString();
        } catch (Exception e) {
            AppLog.w(TAG, "JPush: failed to build JSON for push-registered event", e);
            return;
        }
        MainActivity.instance.runOnUiThread(() -> {
            // Update pushAvailable if not already set
            if (!MainActivity.instance.pushAvailable) {
                MainActivity.instance.pushAvailable = true;
            }
            // Dispatch a custom event to the WebView so useGlobalEvents can register via WS
            if (MainActivity.instance.webView != null) {
                MainActivity.instance.webView.evaluateJavascript(
                    "window.dispatchEvent(new CustomEvent('clawbench-push-registered', { detail: " + jsArg + " }))",
                    null
                );
            }
        });
    }
}
