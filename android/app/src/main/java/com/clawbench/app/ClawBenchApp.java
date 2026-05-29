package com.clawbench.app;

import android.app.Application;
import android.content.Intent;
import android.os.Build;
import android.webkit.WebView;
import android.util.Log;

/**
 * Custom Application to handle multi-process initialization.
 *
 * The app runs three processes:
 * - :default  — MainActivity with the main ClawBench WebView
 * - :pushcore — JPush push service (PushService + UserService)
 * - :browser  — BrowserActivity with the sandbox WebView
 *
 * Android 9+ (API 28) prohibits multiple processes from using the same
 * WebView data directory. This class detects the :browser process and
 * assigns it a separate data directory suffix via WebView.setDataDirectorySuffix().
 *
 * In the :pushcore process, this class starts PushService as a foreground service
 * to keep the process alive with a persistent notification.
 *
 * JPush initialization is NOT done here — it's deferred to MainActivity
 * which first fetches the AppKey from the server's /api/push/config endpoint.
 */
public class ClawBenchApp extends Application {

    private static final String TAG = "ClawBench";

    @Override
    public void onCreate() {
        super.onCreate();

        if (isBrowserProcess()) {
            // Must be called before any WebView is created in this process
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                try {
                    WebView.setDataDirectorySuffix("browser");
                    AppLog.i(TAG, "Browser process: set WebView data directory suffix");
                } catch (Exception e) {
                    AppLog.e(TAG, "Failed to set WebView data directory suffix", e);
                }
            }
        } else if (isPushCoreProcess()) {
            // Start PushService to keep :pushcore alive with a persistent notification.
            // PushService calls startForeground() in its onCreate(), promoting itself
            // to a foreground service that the system won't easily kill.
            // Use startForegroundService() on API 26+ to satisfy the 5-second
            // startForeground() obligation — PushService handles this in onCreate().
            AppLog.i(TAG, "Pushcore process: starting PushService");
            Intent pushIntent = new Intent(this, PushService.class);
            try {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    startForegroundService(pushIntent);
                } else {
                    startService(pushIntent);
                }
            } catch (Exception e) {
                AppLog.w(TAG, "Failed to start PushService: " + e.getMessage());
            }
        }
        // JPush init is done in MainActivity.fetchPushConfig() after getting AppKey from server
    }

    /**
     * Check if the current process is the :browser sandbox process.
     * Detects by checking whether the process name ends with ":browser".
     */
    private boolean isBrowserProcess() {
        return getProcessNameSuffix().equals(":browser");
    }

    /**
     * Check if the current process is the :pushcore JPush process.
     * Detects by checking whether the process name ends with ":pushcore".
     */
    private boolean isPushCoreProcess() {
        return getProcessNameSuffix().equals(":pushcore");
    }

    /**
     * Get the suffix of the current process name (e.g. ":browser", ":pushcore", or "" for main).
     */
    private String getProcessNameSuffix() {
        try {
            String processName = "";
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                processName = getProcessName();
            } else {
                // Fallback for pre-API 28: read from /proc/self/cmdline
                java.io.BufferedReader reader = new java.io.BufferedReader(
                    new java.io.FileReader("/proc/self/cmdline"));
                processName = reader.readLine().trim();
                reader.close();
            }
            // Process name is like "com.clawbench.app:pushcore"
            int colon = processName.lastIndexOf(':');
            return colon >= 0 ? processName.substring(colon) : "";
        } catch (Exception e) {
            AppLog.w(TAG, "Failed to detect process name", e);
            return "";
        }
    }
}
