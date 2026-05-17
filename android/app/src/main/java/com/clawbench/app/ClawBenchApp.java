package com.clawbench.app;

import android.app.Application;
import android.os.Build;
import android.webkit.WebView;
import android.util.Log;

/**
 * Custom Application to handle multi-process WebView initialization.
 *
 * The app runs two processes:
 * - :default  — MainActivity with the main ClawBench WebView
 * - :browser  — BrowserActivity with the sandbox WebView
 *
 * Android 9+ (API 28) prohibits multiple processes from using the same
 * WebView data directory. This class detects the :browser process and
 * assigns it a separate data directory suffix via WebView.setDataDirectorySuffix().
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
                    Log.i(TAG, "Browser process: set WebView data directory suffix");
                } catch (Exception e) {
                    Log.e(TAG, "Failed to set WebView data directory suffix", e);
                }
            }
        }
        // JPush init is done in MainActivity.fetchPushConfig() after getting AppKey from server
    }

    /**
     * Check if the current process is the :browser sandbox process.
     * Detects by checking whether the process name ends with ":browser".
     */
    private boolean isBrowserProcess() {
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
            return processName.endsWith(":browser");
        } catch (Exception e) {
            Log.w(TAG, "Failed to detect process name", e);
            return false;
        }
    }
}
