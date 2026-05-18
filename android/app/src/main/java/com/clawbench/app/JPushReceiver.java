package com.clawbench.app;

import android.content.Context;
import android.content.SharedPreferences;
import android.util.Log;
import cn.jpush.android.api.NotificationMessage;
import cn.jpush.android.service.JPushMessageReceiver;

public class JPushReceiver extends JPushMessageReceiver {
    private static final String TAG = "ClawBench";
    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";

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
        if (sessionId != null && MainActivity.instance != null) {
            MainActivity.instance.runOnUiThread(() -> {
                if (MainActivity.instance.webView != null) {
                    MainActivity.instance.webView.evaluateJavascript(
                        "window.dispatchEvent(new CustomEvent('clawbench-open-session', { detail: { sessionId: '" + sessionId + "' } }))",
                        null
                    );
                }
            });
        }
    }

    @Override
    public void onRegister(Context context, String registrationId) {
        Log.i(TAG, "JPush registered: " + registrationId);
        // Immediately register the ID with the server via HTTP.
        // This fixes the timing race where registerPushId() in the WebView
        // is called before JPush SDK has finished registering.
        registerPushIdWithServer(context, registrationId);
        // Also notify the WebView layer so it can update pushAvailable/pushRegistered state
        notifyWebView(registrationId);
    }

    @Override
    public void onConnected(Context context, boolean isConnected) {
        Log.i(TAG, "JPush connected: " + isConnected);
    }

    /**
     * Send the JPush Registration ID to the ClawBench server via HTTP POST.
     * This runs on a background thread to avoid blocking the JPush callback.
     * The server stores the reg ID so it can send push notifications when WS is disconnected.
     */
    private void registerPushIdWithServer(Context context, String registrationId) {
        if (registrationId == null || registrationId.isEmpty()) return;

        SharedPreferences prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
        String serverUrl = prefs.getString(KEY_SERVER_URL, "");
        if (serverUrl.isEmpty()) {
            Log.w(TAG, "No server URL configured, cannot register push ID");
            return;
        }

        new Thread(() -> {
            try {
                java.net.URL url = new java.net.URL(serverUrl + "/api/push/register");
                java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();
                conn.setRequestMethod("POST");
                conn.setDoOutput(true);
                conn.setRequestProperty("Content-Type", "application/json");
                conn.setConnectTimeout(5000);
                conn.setReadTimeout(5000);

                // Carry auth cookies (the WebView's login session)
                String cookies = android.webkit.CookieManager.getInstance().getCookie(serverUrl);
                if (cookies != null) {
                    conn.setRequestProperty("Cookie", cookies);
                }

                String payload = "{\"registration_id\":\"" + registrationId + "\"}";
                java.io.OutputStream os = conn.getOutputStream();
                os.write(payload.getBytes("UTF-8"));
                os.close();

                int code = conn.getResponseCode();
                if (code >= 200 && code < 300) {
                    Log.i(TAG, "Push ID registered with server successfully");
                } else {
                    Log.w(TAG, "Push ID registration failed: HTTP " + code);
                }
                conn.disconnect();
            } catch (Exception e) {
                Log.w(TAG, "Failed to register push ID with server: " + e.getMessage());
            }
        }).start();
    }

    /**
     * Notify the WebView layer that the JPush Registration ID is now available.
     * This triggers a re-check of push availability and re-registration via the normal path.
     */
    private void notifyWebView(String registrationId) {
        if (MainActivity.instance == null) return;
        MainActivity.instance.runOnUiThread(() -> {
            // Update pushAvailable if not already set
            if (!MainActivity.instance.pushAvailable) {
                MainActivity.instance.pushAvailable = true;
            }
            // Dispatch a custom event to the WebView so useGlobalEvents can re-register
            if (MainActivity.instance.webView != null) {
                MainActivity.instance.webView.evaluateJavascript(
                    "window.dispatchEvent(new CustomEvent('clawbench-push-registered', { detail: { registrationId: '" + registrationId + "' } }))",
                    null
                );
            }
        });
    }
}
