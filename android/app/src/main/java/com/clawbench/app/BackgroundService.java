package com.clawbench.app;

import android.app.Notification;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.app.AlarmManager;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.PowerManager;
import android.os.SystemClock;
import android.content.pm.ServiceInfo;
import android.util.Log;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;

import com.jcraft.jsch.JSch;
import com.jcraft.jsch.Session;

import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URL;
import java.security.SecureRandom;
import java.security.cert.X509Certificate;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import javax.net.ssl.HttpsURLConnection;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.WebSocket;
import okhttp3.WebSocketListener;

/**
 * Foreground service for background connectivity.
 *
 * Manages:
 * 1. SSH tunnels for port forwarding (127.0.0.1:{port} on device → 127.0.0.1:{port} on server)
 * 2. Native WebSocket event channel for AI session/task notifications when JPush is unavailable
 * 3. JPush push notification integration for real-time event delivery
 *
 * The service stays alive as long as at least one of these is active:
 * - Any forwarded SSH ports
 * - Native WS needed (JPush unavailable)
 *
 * Reliability features:
 * - Auto-reconnect: monitors SSH connection and reconnects with exponential backoff
 * - Port persistence: saves forwarded ports to SharedPreferences, restores on Service restart
 * - WifiLock: prevents WiFi from disconnecting while SSH tunnel is active
 * - WakeLock: prevents CPU from sleeping so SSH keep-alive packets are sent
 *
 * All SSH/HTTP network operations run on a background thread to avoid NetworkOnMainThreadException.
 */
public class BackgroundService extends Service {

    private static final String TAG = "ClawBench";
    private static final int NOTIFICATION_ID = 2;
    private static final String CHANNEL_ID = "clawbench_background";
    private static final String EVENTS_CHANNEL_ID = "clawbench_events";
    private static final int EVENTS_NOTIFICATION_ID = 3;
    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";
    private static final String KEY_SSH_PASSWORD = "ssh_password";
    private static final String KEY_FORWARDED_PORTS = "forwarded_ports";
    private static final String KEY_BATTERY_OPT_REQUESTED = "battery_opt_requested";

    // Reconnect parameters: exponential backoff delays in milliseconds
    private static final int[] RECONNECT_DELAYS_MS = {5000, 10000, 30000, 60000, 120000};
    private static final int MAX_RECONNECT_ATTEMPTS = 10;
    private static final int MONITOR_CHECK_INTERVAL_MS = 15000;

    private static volatile boolean isRunning = false;
    private static volatile BackgroundService instance;

    private JSch jsch;
    private Session sshSession;
    private final Set<Integer> forwardedPorts = ConcurrentHashMap.newKeySet();
    private String serverHost;
    private int sshPort;
    private String password;

    // Background thread for all network I/O (SSH connect, HTTP fetch, port forward)
    private final ExecutorService networkExecutor = Executors.newSingleThreadExecutor();

    // Lazily initialized SSL context that trusts all certs (for self-signed ClawBench servers)
    private static SSLContext trustAllSSLContext;

    // Connection monitor: watches sshSession.isConnected() and triggers reconnect
    private Thread connectionMonitor;
    private volatile boolean monitorActive = false;
    private volatile boolean intentionalDisconnect = false;

    // WifiLock: prevents WiFi from being disabled while SSH tunnel is active
    private WifiManager.WifiLock wifiLock;

    // WakeLock: prevents CPU from sleeping so SSH keep-alive packets are sent
    private PowerManager.WakeLock wakeLock;

    // Reconnect state
    private volatile boolean isReconnecting = false;
    private volatile int reconnectAttempt = 0;

    // Last SSH error message (for JS bridge error reporting)
    private static volatile String lastError = null;

    // --- Native WebSocket for background event notifications (when JPush is not available) ---
    private WebSocket nativeEventWs;
    private volatile boolean nativeWsActive = false;
    private volatile boolean nativeWsIntentionalStop = false;
    private volatile int nativeWsReconnectAttempt = 0;
    private String nativeClientId;
    // Tracks whether the native WS needs this Service to stay alive.
    // Without this flag, onCreate() would stopSelf() when there are no SSH ports,
    // killing the Service before the native WS can be established.
    // Must be static so startNativeEventWs() can set it before the Service is created.
    private static volatile boolean nativeWsNeeded = false;

    public static boolean isRunning() {
        return isRunning;
    }

    /**
     * Check whether the SSH tunnel is currently connected.
     * Can be called from any thread (used by WebAppInterface for JS bridge).
     * Returns false if the service is not running or the session is disconnected.
     */
    public static boolean isTunnelConnected() {
        // Access via static reference is inherently racy, but sufficient for
        // a health-check ping — worst case we return a stale value that gets
        // corrected on the next poll.
        boolean connected = isRunning && instance != null && instance.sshSession != null && instance.sshSession.isConnected();
        if (connected) {
            lastError = null;
        }
        return connected;
    }

    /**
     * Get the last SSH connection error message.
     * Returns null if no error (tunnel connected or never attempted).
     * Can be called from any thread (used by WebAppInterface for JS bridge).
     */
    public static String getLastError() {
        return lastError;
    }

    /**
     * Classify an SSH error into a short, user-friendly error code.
     * Used by the JS bridge to provide localized error messages in the frontend.
     * Returns one of: "auth", "network", "server", "unknown", or null.
     */
    public static String getErrorType() {
        String err = lastError;
        if (err == null) return null;
        err = err.toLowerCase();
        if (err.contains("auth") || err.contains("password") || err.contains("permission")
                || err.contains("credential") || err.contains("denied")) {
            return "auth";
        }
        if (err.contains("timeout") || err.contains("refused") || err.contains("unreachable")
                || err.contains("no route") || err.contains("network")
                || err.contains("connection reset") || err.contains("broken pipe")
                || err.contains("eof") || err.contains("reset")) {
            return "network";
        }
        if (err.contains("host key") || err.contains("fingerprint")) {
            return "hostkey";
        }
        return "unknown";
    }

    /**
     * Start the port forward service.
     */
    public static void start(Context context) {
        if (isRunning) return;
        Intent intent = new Intent(context, BackgroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    /**
     * Stop the port forward service and disconnect SSH.
     */
    public static void stop(Context context) {
        if (!isRunning) return;
        Intent intent = new Intent(context, BackgroundService.class);
        context.stopService(intent);
    }

    /**
     * Save the SSH password to SharedPreferences.
     * Called from WebAppInterface when user logs in via WebView.
     */
    public static void setPassword(Context context, String password) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putString(KEY_SSH_PASSWORD, password)
                .apply();
    }

    /**
     * Check whether battery optimization has already been requested.
     */
    public static boolean isBatteryOptRequested(Context context) {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .getBoolean(KEY_BATTERY_OPT_REQUESTED, false);
    }

    /**
     * Mark battery optimization as requested (so we don't ask again).
     */
    public static void setBatteryOptRequested(Context context) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putBoolean(KEY_BATTERY_OPT_REQUESTED, true)
                .apply();
    }


    @Override
    public void onCreate() {
        super.onCreate();
        isRunning = true;
        instance = this;
        jsch = new JSch();
        createNotificationChannel();
        startForegroundCompat(NOTIFICATION_ID, buildNotification(0, null));

        // Restore previously saved ports (from before Service was killed)
        restoreForwardedPorts();

        // If no ports were restored and native WS is not needed, there's nothing
        // to do — stop immediately to avoid wasting battery on an idle foreground service.
        // When native WS is needed (JPush not available), the Service must stay alive
        // so the background WebSocket can receive events and post local notifications.
        if (forwardedPorts.isEmpty() && !nativeWsNeeded) {
            AppLog.i(TAG, "SSH: no saved ports to forward and native WS not needed, stopping service");
            stopSelf();
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (!isRunning) {
            isRunning = true;
            startForegroundCompat(NOTIFICATION_ID, buildNotification(0, null));
        }

        if (intent != null) {
            String action = intent.getAction();
            if ("ADD_PORT".equals(action)) {
                int port = intent.getIntExtra("port", 0);
                if (port > 0) {
                    networkExecutor.execute(() -> addPortForward(port));
                }
            } else if ("REMOVE_PORT".equals(action)) {
                int port = intent.getIntExtra("port", 0);
                if (port > 0) {
                    networkExecutor.execute(() -> removePortForward(port));
                }
            } else if ("DISCONNECT".equals(action)) {
                networkExecutor.execute(this::disconnect);
            } else if ("RESTORE_PORTS".equals(action)) {
                // Explicit restore request — e.g. from Activity after configuration change
                networkExecutor.execute(this::restoreAndReconnect);
            } else if ("START_NATIVE_WS".equals(action)) {
                nativeWsNeeded = true;
                networkExecutor.execute(() -> {
                    String serverUrl = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                            .getString(KEY_SERVER_URL, "");
                    if (!serverUrl.isEmpty()) {
                        startNativeEventWs(serverUrl);
                    }
                });
            } else if ("STOP_NATIVE_WS".equals(action)) {
                networkExecutor.execute(this::stopNativeEventWs);
            }
        } else {
            // START_STICKY restart: Android killed the service and recreated it with null intent.
            // onCreate() already restored port numbers into forwardedPorts via restoreForwardedPorts(),
            // but the SSH session was lost. Re-establish the tunnel now.
            if (!forwardedPorts.isEmpty()) {
                AppLog.i(TAG, "SSH: service restarted by START_STICKY, restoring " + forwardedPorts.size() + " port forwards");
                networkExecutor.execute(this::restoreAndReconnect);
            }
            // Also restore native WS if it was active before the service was killed.
            // Without this, background push notifications are lost after Android kills the service.
            if (nativeWsNeeded) {
                AppLog.i(TAG, "NativeWS: service restarted by START_STICKY, restoring native WS");
                networkExecutor.execute(() -> {
                    String serverUrl = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                            .getString(KEY_SERVER_URL, "");
                    if (!serverUrl.isEmpty()) {
                        startNativeEventWs(serverUrl);
                    }
                });
            }
        }

        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        intentionalDisconnect = true;
        stopNativeEventWs();
        stopConnectionMonitor();
        releaseWifiLock();
        releaseWakeLock();
        disconnectInternal();
        isRunning = false;
        instance = null;
        networkExecutor.shutdownNow();

        // If there are no forwarded ports, clean up SharedPreferences
        // so the service won't be unnecessarily restored on next cold start.
        if (forwardedPorts.isEmpty()) {
            getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                    .edit()
                    .remove(KEY_FORWARDED_PORTS)
                    .apply();
            AppLog.i(TAG, "SSH: cleaned up empty forwarded_ports from SharedPreferences");
        }

        stopForeground(true);
        super.onDestroy();
    }

    @Override
    public void onTaskRemoved(Intent rootIntent) {
        // When the user swipes the app from recents, restart the service
        // so the SSH tunnel keeps running.
        Intent restartIntent = new Intent(this, BackgroundService.class);
        restartIntent.setAction("RESTORE_PORTS");
        PendingIntent pendingIntent = PendingIntent.getService(
                this, 1, restartIntent,
                PendingIntent.FLAG_ONE_SHOT | PendingIntent.FLAG_IMMUTABLE);
        AlarmManager alarmManager = (AlarmManager) getSystemService(Context.ALARM_SERVICE);
        if (alarmManager != null) {
            alarmManager.set(AlarmManager.ELAPSED_REALTIME_WAKEUP,
                    SystemClock.elapsedRealtime() + 1000, pendingIntent);
        }
        super.onTaskRemoved(rootIntent);
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    // --- Port list persistence ---

    /**
     * Save the current forwarded ports set to SharedPreferences.
     */
    private void saveForwardedPorts() {
        Set<String> portStrings = new HashSet<>();
        for (int port : forwardedPorts) {
            portStrings.add(String.valueOf(port));
        }
        getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                .edit()
                .putStringSet(KEY_FORWARDED_PORTS, portStrings)
                .apply();
    }

    /**
     * Restore forwarded ports from SharedPreferences (without actually connecting).
     * The actual SSH connection and port forward setup happens when restoreAndReconnect() is called.
     */
    private void restoreForwardedPorts() {
        Set<String> portStrings = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                .getStringSet(KEY_FORWARDED_PORTS, null);
        if (portStrings != null && !portStrings.isEmpty()) {
            for (String ps : portStrings) {
                try {
                    int port = Integer.parseInt(ps);
                    forwardedPorts.add(port);
                } catch (NumberFormatException ignored) {}
            }
            AppLog.i(TAG, "SSH: restored " + forwardedPorts.size() + " forwarded ports from prefs");
            updateNotification(forwardedPorts.size(), null);
        }
    }

    /**
     * Restore ports and reconnect SSH — called after START_STICKY restart.
     */
    private void restoreAndReconnect() {
        if (forwardedPorts.isEmpty()) {
            restoreForwardedPorts();
        }
        if (!forwardedPorts.isEmpty()) {
            try {
                ensureConnection();
                AppLog.i(TAG, "SSH: restored all port forwards after service restart");
            } catch (Exception e) {
                lastError = e.getMessage();
                AppLog.e(TAG, "SSH: failed to restore connection after service restart", e);
                // Connection monitor will handle reconnect
            }
        }
    }

    // --- Connection monitor (auto-reconnect) ---

    /**
     * Start the connection monitor thread.
     * Periodically checks if the SSH session is still alive and triggers reconnect if not.
     */
    private void startConnectionMonitor() {
        if (monitorActive) return;
        monitorActive = true;

        connectionMonitor = new Thread(() -> {
            AppLog.i(TAG, "SSH: connection monitor started");
            while (monitorActive && !Thread.currentThread().isInterrupted()) {
                try {
                    Thread.sleep(MONITOR_CHECK_INTERVAL_MS);
                } catch (InterruptedException e) {
                    break;
                }

                if (!monitorActive || intentionalDisconnect) break;

                // Check if session is dead
                if (sshSession == null || !sshSession.isConnected()) {
                    if (forwardedPorts.isEmpty()) {
                        // No ports to maintain — don't bother reconnecting
                        AppLog.d(TAG, "SSH: session disconnected but no ports to forward, skipping reconnect");
                        continue;
                    }

                    AppLog.w(TAG, "SSH: session disconnected, starting auto-reconnect");
                    isReconnecting = true;
                    reconnectAttempt = 0;
                    updateNotification(forwardedPorts.size(), "SSH 隧道断开，正在重连…");

                    while (monitorActive && !intentionalDisconnect && !Thread.currentThread().isInterrupted()) {
                        reconnectAttempt++;
                        int delayIdx = Math.min(reconnectAttempt - 1, RECONNECT_DELAYS_MS.length - 1);
                        int delay = RECONNECT_DELAYS_MS[delayIdx];

                        // Wait before attempt (except first attempt)
                        if (reconnectAttempt > 1) {
                            updateNotification(forwardedPorts.size(),
                                    "SSH 隧道断开，第 " + reconnectAttempt + "/" + MAX_RECONNECT_ATTEMPTS + " 次重连…");
                            try {
                                Thread.sleep(delay);
                            } catch (InterruptedException e) {
                                break;
                            }
                        }

                        if (!monitorActive || intentionalDisconnect) break;

                        // Give up after max attempts
                        if (reconnectAttempt > MAX_RECONNECT_ATTEMPTS) {
                            AppLog.e(TAG, "SSH: exhausted " + MAX_RECONNECT_ATTEMPTS + " reconnect attempts, giving up");
                            updateNotification(forwardedPorts.size(), "SSH 隧道重连失败，请重新打开页面");
                            break;
                        }

                        try {
                            AppLog.i(TAG, "SSH: auto-reconnect attempt #" + reconnectAttempt);
                            ensureConnection();
                            lastError = null;
                            AppLog.i(TAG, "SSH: auto-reconnect succeeded on attempt #" + reconnectAttempt);
                            isReconnecting = false;
                            reconnectAttempt = 0;
                            updateNotification(forwardedPorts.size(), "SSH 隧道已恢复");
                            // Clear the "recovered" status after 3 seconds
                            try {
                                Thread.sleep(3000);
                            } catch (InterruptedException e) {
                                break;
                            }
                            if (monitorActive && !isReconnecting) {
                                updateNotification(forwardedPorts.size(), null);
                            }
                            break; // Reconnected successfully
                        } catch (Exception e) {
                            lastError = e.getMessage();
                            AppLog.w(TAG, "SSH: auto-reconnect attempt #" + reconnectAttempt + " failed: " + e.getMessage());
                        }
                    }

                    if (isReconnecting) {
                        // Exhausted all attempts or monitor stopped
                        isReconnecting = false;
                    }
                }
            }
            AppLog.i(TAG, "SSH: connection monitor stopped");
        }, "SSH-ConnectionMonitor");

        connectionMonitor.setDaemon(true);
        connectionMonitor.start();
    }

    /**
     * Stop the connection monitor thread.
     */
    private void stopConnectionMonitor() {
        monitorActive = false;
        if (connectionMonitor != null) {
            connectionMonitor.interrupt();
            connectionMonitor = null;
        }
    }

    // --- WifiLock ---

    /**
     * Acquire a WifiLock to prevent WiFi from disconnecting while the SSH tunnel is active.
     */
    private void acquireWifiLock() {
        if (wifiLock != null && wifiLock.isHeld()) return;
        try {
            WifiManager wifiManager = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wifiManager != null) {
                // WIFI_MODE_FULL_HIGH_PERF uses less power than WIFI_MODE_FULL (Android 12+)
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                    wifiLock = wifiManager.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "ClawBench-SSH");
                } else {
                    wifiLock = wifiManager.createWifiLock(WifiManager.WIFI_MODE_FULL, "ClawBench-SSH");
                }
                wifiLock.setReferenceCounted(false);
                wifiLock.acquire();
                AppLog.i(TAG, "SSH: WifiLock acquired");
            }
        } catch (Exception e) {
            AppLog.w(TAG, "SSH: failed to acquire WifiLock", e);
        }
    }

    /**
     * Release the WifiLock.
     */
    private void releaseWifiLock() {
        if (wifiLock != null && wifiLock.isHeld()) {
            try {
                wifiLock.release();
                AppLog.i(TAG, "SSH: WifiLock released");
            } catch (Exception e) {
                AppLog.w(TAG, "SSH: failed to release WifiLock", e);
            }
            wifiLock = null;
        }
    }

    /**
     * Ensure SSH connection is established. Connects if not already connected.
     * On successful connection, starts the connection monitor and acquires WifiLock.
     * MUST be called from a background thread (network I/O).
     */
    private synchronized void ensureConnection() throws Exception {
        if (sshSession != null && sshSession.isConnected()) {
            return;
        }

        // Load server configuration from SharedPreferences
        SharedPreferences prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);
        String serverUrl = prefs.getString(KEY_SERVER_URL, "");
        if (serverUrl.isEmpty()) {
            throw new Exception("Server URL not configured");
        }

        // Parse server host and determine SSH port
        Uri uri = Uri.parse(serverUrl);
        serverHost = uri.getHost();
        int httpPort = uri.getPort();
        if (httpPort < 0) {
            httpPort = serverUrl.startsWith("https://") ? 443 : 80;
        }

        // Fetch SSH port from /api/ssh/info endpoint
        sshPort = fetchSSHPort(serverUrl, httpPort);
        if (sshPort <= 0) {
            throw new Exception("Failed to get SSH port from server. Please check that SSH tunnel is enabled in server config.");
        }

        // Get password from SharedPreferences (set by WebAppInterface on login)
        password = prefs.getString(KEY_SSH_PASSWORD, "");
        if (password.isEmpty()) {
            throw new Exception("SSH password not configured. Please log in first.");
        }

        AppLog.i(TAG, "SSH: connecting to " + serverHost + ":" + sshPort);

        // Create SSH session
        sshSession = jsch.getSession("clawbench", serverHost, sshPort);
        sshSession.setPassword(password);
        sshSession.setConfig("StrictHostKeyChecking", "no");
        sshSession.setConfig("PreferredAuthentications", "password");
        sshSession.setServerAliveInterval(30000); // 30s keep-alive
        sshSession.setServerAliveCountMax(3);
        sshSession.setTimeout(15000); // 15s connection timeout

        sshSession.connect(15000); // 15s connection timeout

        // Connection succeeded — clear any previous error
        lastError = null;

        AppLog.i(TAG, "SSH: connected to " + serverHost + ":" + sshPort);

        // Acquire WifiLock to prevent WiFi from being disabled
        acquireWifiLock();

        // Acquire WakeLock to prevent CPU from sleeping (keeps SSH keep-alive working)
        acquireWakeLock();

        // Re-establish any previously forwarded ports
        int reEstablished = 0;
        for (int port : forwardedPorts) {
            try {
                sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
                reEstablished++;
                AppLog.i(TAG, "SSH: re-established port forward " + port);
            } catch (Exception e) {
                AppLog.e(TAG, "SSH: failed to re-establish port forward " + port, e);
            }
        }
        AppLog.i(TAG, "SSH: re-established " + reEstablished + "/" + forwardedPorts.size() + " port forwards");

        updateNotification(forwardedPorts.size(), null);

        // Start connection monitor to detect future disconnects
        startConnectionMonitor();
    }

    /**
     * Add a local port forward through the SSH tunnel.
     * Creates: 127.0.0.1:{port} on device → 127.0.0.1:{port} on server
     *
     * If the port is already in the forwarded set but the SSH session is disconnected,
     * this will reconnect and re-establish the port forward. This handles the case where
     * the Go backend restarts (killing the SSH server), and the Android frontend calls
     * syncToNative() → addForwardedPort() on page reload — the port is in the set
     * but the tunnel is dead.
     *
     * MUST be called from a background thread (network I/O).
     */
    private synchronized void addPortForward(int port) {
        boolean alreadyInSet = forwardedPorts.contains(port);
        boolean sessionAlive = sshSession != null && sshSession.isConnected();

        if (alreadyInSet && sessionAlive) {
            // Port is tracked and SSH session is alive — nothing to do.
            // (ensureConnection() already re-established this forward when it reconnected.)
            AppLog.d(TAG, "SSH: port " + port + " already forwarded and session active");
            return;
        }

        // Port is in the set but session is dead — need to reconnect.
        // Or port is new — need to add it.
        if (alreadyInSet && !sessionAlive) {
            AppLog.i(TAG, "SSH: port " + port + " in set but session disconnected, reconnecting...");
        }

        try {
            ensureConnection();
            // ensureConnection() rebuilds ALL ports in forwardedPorts when it reconnects.
            // For new ports (not in set yet), we need to explicitly set up forwarding.
            if (!alreadyInSet) {
                sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
                forwardedPorts.add(port);
                saveForwardedPorts();
            }
            // For already-tracked ports, ensureConnection() already called
            // setPortForwardingL for all ports in forwardedPorts during reconnect.
            // If ensureConnection() failed to set up this specific port, it logged
            // the error — the connection monitor will retry later.
            AppLog.i(TAG, "SSH: port forward ready: localhost:" + port + " → server:" + port);
            updateNotification(forwardedPorts.size(), null);
        } catch (Exception e) {
            lastError = e.getMessage();
            AppLog.e(TAG, "SSH: failed to add port forward for " + port + ", retrying...", e);
            // Disconnect and retry once (password may have been updated, or session stale)
            disconnectInternal();
            try {
                ensureConnection();
                if (!alreadyInSet) {
                    sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
                    forwardedPorts.add(port);
                    saveForwardedPorts();
                }
                AppLog.i(TAG, "SSH: port forward added on retry: localhost:" + port + " → server:" + port);
                updateNotification(forwardedPorts.size(), null);
            } catch (Exception e2) {
                lastError = e2.getMessage();
                AppLog.e(TAG, "SSH: failed to add port forward for " + port + " on retry", e2);
            }
        }
    }

    /**
     * Remove a local port forward.
     */
    synchronized void removePortForward(int port) {
        if (!forwardedPorts.contains(port)) {
            return;
        }

        try {
            if (sshSession != null && sshSession.isConnected()) {
                sshSession.delPortForwardingL(port);
                AppLog.i(TAG, "SSH: port forward removed: " + port);
            }
        } catch (Exception e) {
            AppLog.e(TAG, "SSH: failed to remove port forward for " + port, e);
        }

        forwardedPorts.remove(port);
        saveForwardedPorts();
        updateNotification(forwardedPorts.size(), null);

        // If no more forwarded ports and native WS is not needed, stop the service
        if (forwardedPorts.isEmpty() && !nativeWsNeeded) {
            stopSelf();
        }
    }

    /**
     * Fetch SSH port from the /api/ssh/info endpoint.
     * Handles self-signed HTTPS certificates (ClawBench often uses Let's Encrypt or self-signed certs).
     * Returns the port number, or -1 on failure.
     * MUST be called from a background thread (network I/O).
     */
    private int fetchSSHPort(String serverUrl, int httpPort) {
        try {
            Uri uri = Uri.parse(serverUrl);
            String scheme = uri.getScheme();
            if (scheme == null) scheme = "https";
            String host = uri.getHost();
            String path = scheme + "://" + host + ":" + httpPort + "/api/ssh/info";

            URL url = new URL(path);
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();

            // Handle self-signed HTTPS certificates
            if (conn instanceof HttpsURLConnection && trustAllSSLContext != null) {
                ((HttpsURLConnection) conn).setSSLSocketFactory(trustAllSSLContext.getSocketFactory());
                ((HttpsURLConnection) conn).setHostnameVerifier((hostname, session) -> true);
            }

            conn.setRequestMethod("GET");
            conn.setConnectTimeout(5000);
            conn.setReadTimeout(5000);

            int code = conn.getResponseCode();
            if (code == 200) {
                BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream()));
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = reader.readLine()) != null) {
                    sb.append(line);
                }
                reader.close();

                JSONObject json = new JSONObject(sb.toString());
                boolean enabled = json.optBoolean("enabled", false);
                if (enabled) {
                    int port = json.optInt("port", -1);
                    AppLog.i(TAG, "SSH: fetched SSH port " + port + " from /api/ssh/info");
                    return port;
                } else {
                    AppLog.w(TAG, "SSH: SSH server is not enabled on the server");
                    return -1;
                }
            } else {
                AppLog.w(TAG, "SSH: /api/ssh/info returned HTTP " + code);
                return -1;
            }
        } catch (Exception e) {
            AppLog.w(TAG, "SSH: failed to fetch SSH info, will fallback to httpPort+1", e);
            return -1;
        }
    }

    /**
     * Disconnect the SSH session (user-initiated, stops reconnect).
     * Clears port list and stops the service.
     */
    private synchronized void disconnect() {
        intentionalDisconnect = true;
        stopConnectionMonitor();
        releaseWifiLock();
        releaseWakeLock();
        disconnectInternal();
    }

    /**
     * Internal disconnect: tears down SSH session but does NOT affect monitor/wifi lock.
     * Used by ensureConnection retry logic (disconnect old session before reconnecting).
     * Note: does NOT clear forwardedPorts — we want to preserve them for reconnect.
     */
    private synchronized void disconnectInternal() {
        if (sshSession != null) {
            try {
                // Remove all port forwards before disconnecting
                for (int port : new HashSet<>(forwardedPorts)) {
                    try {
                        sshSession.delPortForwardingL(port);
                    } catch (Exception ignored) {}
                }
                sshSession.disconnect();
                AppLog.i(TAG, "SSH: disconnected");
            } catch (Exception e) {
                AppLog.e(TAG, "SSH: error during disconnect", e);
            }
            sshSession = null;
        }
    }

    private void updateNotification(int portCount, String statusText) {
        NotificationManager nm = getSystemService(NotificationManager.class);
        if (nm != null) {
            nm.notify(NOTIFICATION_ID, buildNotification(portCount, statusText));
        }
    }

    /**
     * Create the notification channel (called once in onCreate).
     */
    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            android.app.NotificationManager nm = getSystemService(android.app.NotificationManager.class);
            if (nm != null) {
                // Background connectivity channel (low priority, no sound)
                android.app.NotificationChannel channel = new android.app.NotificationChannel(
                        CHANNEL_ID,
                        "后台连接服务",
                        android.app.NotificationManager.IMPORTANCE_LOW
                );
                channel.setDescription("SSH 端口转发与后台事件监听");
                nm.createNotificationChannel(channel);

                // AI events channel (high priority, sound + vibration)
                android.app.NotificationChannel eventsChannel = new android.app.NotificationChannel(
                        EVENTS_CHANNEL_ID,
                        "AI 事件通知",
                        android.app.NotificationManager.IMPORTANCE_HIGH
                );
                eventsChannel.setDescription("AI会话和任务完成通知");
                eventsChannel.enableLights(true);
                eventsChannel.setVibrationPattern(new long[]{0, 300, 200, 300});
                nm.createNotificationChannel(eventsChannel);
            }
        }
    }

    /**
     * Build the foreground service notification.
     * @param portCount  Number of currently forwarded ports
     * @param statusText Optional status override (e.g. "后台连接断开，正在重连…"). Null = normal status.
     */
    Notification buildNotification(int portCount, String statusText) {
        Intent notificationIntent = new Intent(this, MainActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
                this, 0, notificationIntent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );

        String title;
        String text;
        if (statusText != null) {
            title = "ClawBench";
            text = statusText;
        } else if (portCount > 0) {
            title = "ClawBench";
            text = portCount + " 个端口转发活跃";
        } else if (nativeWsNeeded || nativeWsActive) {
            title = "ClawBench";
            text = "后台事件监听中";
        } else {
            title = "ClawBench";
            text = "后台服务即将停止";
        }

        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle(title)
                .setContentText(text)
                .setSmallIcon(R.drawable.ic_notification)
                .setContentIntent(pendingIntent)
                .setOngoing(true)
                .setSilent(true)
                .build();
    }

    // --- Foreground service compat ---

    /**
     * Start the service as foreground, passing the required foregroundServiceType
     * on Android 14+ (API 34+). Without this, Android 14 throws
     * ForegroundServiceStartNotAllowedException.
     */
    private void startForegroundCompat(int id, Notification notification) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(id, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(id, notification);
        }
    }

    // --- WakeLock ---

    /**
     * Acquire a partial WakeLock to prevent CPU from sleeping.
     * This ensures SSH keep-alive packets are sent even when the screen is off
     * and the device enters Doze mode.
     */
    private void acquireWakeLock() {
        if (wakeLock != null && wakeLock.isHeld()) return;
        try {
            PowerManager pm = (PowerManager) getApplicationContext().getSystemService(Context.POWER_SERVICE);
            if (pm != null) {
                wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "ClawBench:SSH-Tunnel");
                wakeLock.setReferenceCounted(false);
                wakeLock.acquire();
                AppLog.i(TAG, "SSH: WakeLock acquired");
            }
        } catch (Exception e) {
            AppLog.w(TAG, "SSH: failed to acquire WakeLock", e);
        }
    }

    /**
     * Release the WakeLock.
     */
    private void releaseWakeLock() {
        if (wakeLock != null && wakeLock.isHeld()) {
            try {
                wakeLock.release();
                AppLog.i(TAG, "SSH: WakeLock released");
            } catch (Exception e) {
                AppLog.w(TAG, "SSH: failed to release WakeLock", e);
            }
            wakeLock = null;
        }
    }

    // --- Static helper methods for Activity to use ---

    /**
     * Add a port forward via the service.
     */
    public static void addForwardedPort(Context context, int port) {
        Intent intent = new Intent(context, BackgroundService.class);
        intent.setAction("ADD_PORT");
        intent.putExtra("port", port);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    /**
     * Remove a port forward via the service.
     */
    public static void removeForwardedPort(Context context, int port) {
        Intent intent = new Intent(context, BackgroundService.class);
        intent.setAction("REMOVE_PORT");
        intent.putExtra("port", port);
        context.startService(intent);
    }

    /**
     * Initialize the trust-all SSL context for self-signed HTTPS servers.
     * Called once from MainActivity.onCreate().
     */
    public static void initTrustAllSSL() {
        if (trustAllSSLContext != null) return;
        try {
            TrustManager[] trustAllCerts = new TrustManager[]{
                    new X509TrustManager() {
                        public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
                        public void checkClientTrusted(X509Certificate[] certs, String authType) {}
                        public void checkServerTrusted(X509Certificate[] certs, String authType) {}
                    }
            };
            SSLContext sc = SSLContext.getInstance("TLS");
            sc.init(null, trustAllCerts, new SecureRandom());
            trustAllSSLContext = sc;
        } catch (Exception e) {
            AppLog.e(TAG, "SSH: failed to init trust-all SSL context", e);
        }
    }

    // --- Native WebSocket for background event notifications ---

    /**
     * Start the native WebSocket for background event notifications.
     * Called when the app goes to background and JPush is NOT available.
     * Sets nativeWsNeeded BEFORE starting the Service so that onCreate()
     * won't stopSelf() due to having no SSH ports to forward.
     */
    public static void startNativeEventWs(Context context) {
        nativeWsNeeded = true;
        if (!isRunning) {
            // Service not running — start it first
            start(context);
        }
        Intent intent = new Intent(context, BackgroundService.class);
        intent.setAction("START_NATIVE_WS");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    /**
     * Stop the native WebSocket for background event notifications.
     * Called when the app returns to foreground.
     */
    public static void stopNativeEventWs(Context context) {
        Intent intent = new Intent(context, BackgroundService.class);
        intent.setAction("STOP_NATIVE_WS");
        context.startService(intent);
    }

    /**
     * Connect to the server's WebSocket event channel for background notifications.
     * Uses a different client_id than the WebView WS so both can coexist.
     * MUST be called from a background thread (network I/O).
     */
    private void startNativeEventWs(String serverUrl) {
        if (nativeWsActive) {
            AppLog.d(TAG, "NativeWS: already active, skipping");
            return;
        }
        nativeWsIntentionalStop = false;
        nativeWsReconnectAttempt = 0;

        // Build native client_id from ANDROID_ID (stable across service restarts)
        if (nativeClientId == null) {
            String androidId = android.provider.Settings.Secure.getString(
                    getContentResolver(), android.provider.Settings.Secure.ANDROID_ID);
            if (androidId != null && androidId.length() >= 8) {
                nativeClientId = "native-bg-" + androidId.substring(0, 8);
            } else {
                nativeClientId = "native-bg-default";
            }
        }

        connectNativeWs(serverUrl);
    }

    private void connectNativeWs(String serverUrl) {
        try {
            // Read session cookie from WebView's CookieManager
            String cookies = android.webkit.CookieManager.getInstance().getCookie(serverUrl);
            String sessionCookie = null;
            if (cookies != null) {
                for (String cookie : cookies.split(";")) {
                    String trimmed = cookie.trim();
                    if (trimmed.startsWith("clawbench_session=")) {
                        sessionCookie = trimmed;
                        break;
                    }
                }
            }
            if (sessionCookie == null) {
                AppLog.w(TAG, "NativeWS: no session cookie found, cannot authenticate");
                return;
            }

            // Build WS URL
            String wsUrl = serverUrl
                    .replace("https://", "wss://")
                    .replace("http://", "ws://")
                    + "/api/ai/events/ws?client_id=" + nativeClientId;

            AppLog.i(TAG, "NativeWS: connecting to " + wsUrl);

            // Build OkHttp client
            OkHttpClient.Builder clientBuilder = new OkHttpClient.Builder()
                    .pingInterval(30, TimeUnit.SECONDS)
                    .readTimeout(0, TimeUnit.MILLISECONDS);

            // Handle self-signed certs
            if (trustAllSSLContext != null && wsUrl.startsWith("wss://")) {
                clientBuilder.sslSocketFactory(trustAllSSLContext.getSocketFactory(), new X509TrustManager() {
                    public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
                    public void checkClientTrusted(X509Certificate[] certs, String authType) {}
                    public void checkServerTrusted(X509Certificate[] certs, String authType) {}
                });
                clientBuilder.hostnameVerifier((hostname, session) -> true);
            }

            // Build request with cookie auth
            Request request = new Request.Builder()
                    .url(wsUrl)
                    .header("Cookie", sessionCookie)
                    .build();

            nativeEventWs = clientBuilder.build().newWebSocket(request, new NativeEventListener());
            nativeWsActive = true;
            AppLog.i(TAG, "NativeWS: connection initiated");

        } catch (Exception e) {
            AppLog.e(TAG, "NativeWS: failed to connect", e);
            nativeWsActive = false;
            scheduleNativeWsReconnect(serverUrl);
        }
    }

    /**
     * Stop the native WebSocket connection.
     * If the Service has no other work (no SSH ports), stop the Service entirely
     * to avoid keeping an idle foreground service running.
     */
    void stopNativeEventWs() {
        nativeWsIntentionalStop = true;
        nativeWsActive = false;
        nativeWsNeeded = false;
        if (nativeEventWs != null) {
            try {
                nativeEventWs.close(1000, "foreground");
            } catch (Exception ignored) {}
            nativeEventWs = null;
        }
        AppLog.i(TAG, "NativeWS: stopped");

        // If no SSH ports are forwarded, the Service has no reason to stay alive.
        // Stop it to avoid wasting battery on an idle foreground service.
        if (forwardedPorts.isEmpty()) {
            AppLog.i(TAG, "NativeWS: no SSH ports either, stopping service");
            stopSelf();
        }
    }

    /**
     * Schedule a reconnect attempt for the native WebSocket.
     */
    private void scheduleNativeWsReconnect(String serverUrl) {
        if (nativeWsIntentionalStop) return;
        nativeWsReconnectAttempt++;
        if (nativeWsReconnectAttempt > MAX_RECONNECT_ATTEMPTS) {
            AppLog.w(TAG, "NativeWS: exhausted reconnect attempts, giving up");
            return;
        }
        int delayIdx = Math.min(nativeWsReconnectAttempt - 1, RECONNECT_DELAYS_MS.length - 1);
        int delay = RECONNECT_DELAYS_MS[delayIdx];
        AppLog.i(TAG, "NativeWS: reconnecting in " + delay + "ms (attempt " + nativeWsReconnectAttempt + ")");

        // Use Handler to schedule on main thread, then post to network executor
        new Handler(Looper.getMainLooper()).postDelayed(() -> {
            if (!nativeWsIntentionalStop && isRunning) {
                networkExecutor.execute(() -> connectNativeWs(serverUrl));
            }
        }, delay);
    }

    /**
     * OkHttp WebSocketListener for native background event notifications.
     */
    private class NativeEventListener extends WebSocketListener {
        @Override
        public void onOpen(WebSocket webSocket, Response response) {
            nativeWsActive = true;
            nativeWsReconnectAttempt = 0;
            AppLog.i(TAG, "NativeWS: connected");
        }

        @Override
        public void onMessage(WebSocket webSocket, String text) {
            try {
                JSONObject msg = new JSONObject(text);
                String type = msg.optString("type", "");

                if ("ping".equals(type)) {
                    // Respond to server ping
                    webSocket.send("{\"type\":\"pong\"}");
                    return;
                }

                if (!"event".equals(type)) return;

                // If JPush is available, native WS is no longer needed —
                // disconnect and let JPush handle notifications going forward.
                // Also check jpushEnabledOnServer: even if JPush SDK hasn't finished
                // initializing (pushAvailable=false), the server will send JPush
                // notifications, so we must not show duplicate notifications.
                if (MainActivity.instance != null &&
                        (MainActivity.instance.pushAvailable || MainActivity.instance.jpushEnabledOnServer)) {
                    AppLog.i(TAG, "NativeWS: JPush available (pushAvailable=" + MainActivity.instance.pushAvailable
                            + ", jpushEnabledOnServer=" + MainActivity.instance.jpushEnabledOnServer
                            + "), disconnecting native WS");
                    nativeWsIntentionalStop = true;
                    webSocket.close(1000, "jpush-available");
                    return;
                }

                String eventId = msg.optString("id", "");
                String event = msg.optString("event", "");
                JSONObject data = msg.optJSONObject("data");
                if (data == null) return;

                // Send ack for every event
                if (!eventId.isEmpty()) {
                    JSONObject ack = new JSONObject();
                    ack.put("type", "ack");
                    ack.put("id", eventId);
                    webSocket.send(ack.toString());
                }

                // Only notify for terminal states
                String status = data.optString("status", "");
                boolean shouldNotify = false;

                if ("session_update".equals(event)
                        && ("completed".equals(status) || "cancelled".equals(status))) {
                    shouldNotify = true;
                } else if ("task_update".equals(event)
                        && ("completed".equals(status) || "failed".equals(status) || "cancelled".equals(status))) {
                    shouldNotify = true;
                }

                if (shouldNotify) {
                    postEventNotification(event, data);
                }

            } catch (Exception e) {
                AppLog.w(TAG, "NativeWS: error processing message", e);
            }
        }

        @Override
        public void onClosing(WebSocket webSocket, int code, String reason) {
            webSocket.close(1000, null);
        }

        @Override
        public void onClosed(WebSocket webSocket, int code, String reason) {
            nativeWsActive = false;
            AppLog.i(TAG, "NativeWS: closed (code=" + code + ", reason=" + reason + ")");
            if (!nativeWsIntentionalStop) {
                String serverUrl = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                        .getString(KEY_SERVER_URL, "");
                if (!serverUrl.isEmpty()) {
                    scheduleNativeWsReconnect(serverUrl);
                }
            }
        }

        @Override
        public void onFailure(WebSocket webSocket, Throwable t, Response response) {
            nativeWsActive = false;
            AppLog.w(TAG, "NativeWS: connection failure: " + (t != null ? t.getMessage() : "unknown"));
            if (!nativeWsIntentionalStop) {
                String serverUrl = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                        .getString(KEY_SERVER_URL, "");
                if (!serverUrl.isEmpty()) {
                    scheduleNativeWsReconnect(serverUrl);
                }
            }
        }
    }

    /**
     * Post a system notification for an AI event.
     */
    private void postEventNotification(String eventType, JSONObject data) {
        try {
            String status = data.optString("status", "");
            String sessionId = null;
            String taskId = null;
            String projectPath = null;
            String title = null;
            String text = null;

            AppLog.i(TAG, "NativeWS: postEventNotification called, eventType=" + eventType + ", data=" + data.toString());

            if ("session_update".equals(eventType)) {
                sessionId = data.optString("session_id", "");
                String responsePreview = data.optString("response_preview", "");
                if ("completed".equals(status)) {
                    title = "AI 任务完成";
                    text = responsePreview.isEmpty() ? "AI会话已结束" : responsePreview;
                } else {
                    title = "AI 会话通知";
                    text = "会话已取消";
                }
            } else if ("task_update".equals(eventType)) {
                taskId = data.optString("task_id", "");
                sessionId = data.optString("session_id", null);
                if ("completed".equals(status)) {
                    title = "计划任务完成";
                    text = "任务已完成";
                } else if ("cancelled".equals(status)) {
                    title = "计划任务通知";
                    text = "任务已取消";
                } else {
                    title = "计划任务通知";
                    text = "任务失败";
                }
            } else {
                AppLog.i(TAG, "NativeWS: postEventNotification - unhandled eventType=" + eventType + ", skipping");
                return;
            }

            // Build intent for notification tap — open the app and navigate to session
            Intent intent = new Intent(this, MainActivity.class);
            intent.setAction(android.content.Intent.ACTION_MAIN);
            intent.addCategory(android.content.Intent.CATEGORY_LAUNCHER);
            intent.addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
            if (sessionId != null && !sessionId.isEmpty()) {
                intent.putExtra("session_id", sessionId);
            }
            if (taskId != null && !taskId.isEmpty()) {
                intent.putExtra("task_id", taskId);
            }
            projectPath = data.optString("project_path", "");
            if (projectPath != null && !projectPath.isEmpty()) {
                intent.putExtra("project_path", projectPath);
            }

            AppLog.i(TAG, "NativeWS: notification intent extras: session_id=" + sessionId
                    + ", task_id=" + taskId + ", project_path=" + projectPath);

            PendingIntent pendingIntent = PendingIntent.getActivity(
                    this, 0, intent,
                    PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
            );

            // Use hash of session_id/task_id as notification ID so each gets its own notification
            int notifId = EVENTS_NOTIFICATION_ID;
            if (sessionId != null && !sessionId.isEmpty()) {
                notifId = EVENTS_NOTIFICATION_ID + Math.abs(sessionId.hashCode() % 1000);
            } else if (taskId != null && !taskId.isEmpty()) {
                notifId = EVENTS_NOTIFICATION_ID + 1000 + Math.abs(taskId.hashCode() % 1000);
            }

            Notification notification = new NotificationCompat.Builder(this, EVENTS_CHANNEL_ID)
                    .setContentTitle(title)
                    .setContentText(text)
                    .setSmallIcon(R.drawable.ic_notification)
                    .setContentIntent(pendingIntent)
                    .setAutoCancel(true)
                    .build();

            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) {
                nm.notify(notifId, notification);
            }

            AppLog.i(TAG, "NativeWS: posted notification: " + title + " - " + text);

        } catch (Exception e) {
            AppLog.e(TAG, "NativeWS: failed to post notification", e);
        }
    }
}
