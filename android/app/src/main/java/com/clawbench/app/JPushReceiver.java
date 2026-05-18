package com.clawbench.app;

import android.content.Context;
import android.util.Log;
import cn.jpush.android.api.NotificationMessage;
import cn.jpush.android.service.JPushMessageReceiver;

public class JPushReceiver extends JPushMessageReceiver {
    private static final String TAG = "ClawBench";

    @Override
    public void onNotifyMessageArrived(Context context, NotificationMessage message) {
        Log.i(TAG, "JPush notification arrived: " + message.msgId);
    }

    @Override
    public void onNotifyMessageOpened(Context context, NotificationMessage message) {
        Log.i(TAG, "JPush notification opened: " + message.msgId);
        // Extract session_id from notification extras and notify the WebView
        String sessionId = null;
        if (message.notificationExtras != null) {
            try {
                org.json.JSONObject extras = new org.json.JSONObject(message.notificationExtras);
                sessionId = extras.optString("session_id", null);
            } catch (Exception e) {
                Log.w(TAG, "Failed to parse notification extras", e);
            }
        }
        if (sessionId == null) return;
        // Build safe JSON parameter to prevent XSS injection via sessionId
        final String jsArg;
        try {
            org.json.JSONObject detail = new org.json.JSONObject();
            detail.put("sessionId", sessionId);
            jsArg = detail.toString();
        } catch (Exception e) {
            Log.w(TAG, "Failed to build JSON for open-session event", e);
            return;
        }
        if (MainActivity.instance != null) {
            MainActivity.instance.runOnUiThread(() -> {
                if (MainActivity.instance.webView != null) {
                    MainActivity.instance.webView.evaluateJavascript(
                        "window.dispatchEvent(new CustomEvent('clawbench-open-session', { detail: " + jsArg + " }))",
                        null
                    );
                }
            });
        }
    }

    @Override
    public void onRegister(Context context, String registrationId) {
        Log.i(TAG, "JPush registered: " + registrationId);
        // Notify the WebView layer so it can register the push ID via WS.
        // Push registration is now done via WS "register" message (tied to the
        // WS session), so we don't need a separate HTTP endpoint anymore.
        notifyWebView(registrationId);
    }

    @Override
    public void onConnected(Context context, boolean isConnected) {
        Log.i(TAG, "JPush connected: " + isConnected);
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
            Log.w(TAG, "Failed to build JSON for push-registered event", e);
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
