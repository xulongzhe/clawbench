package com.clawbench.app;

import android.app.Notification;
import android.app.NotificationChannel;
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

import javax.net.ssl.HttpsURLConnection;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;

/**
 * Foreground service that manages SSH tunnels for port forwarding.
 *
 * When the user registers a port for forwarding, this service:
 * 1. Establishes (or reuses) an SSH connection to the ClawBench server
 * 2. Creates a local port forward: 127.0.0.1:{port} on device → 127.0.0.1:{port} on server
 * 3. WebView can then access http://localhost:{port} transparently
 *
 * Reliability features:
 * - Auto-reconnect: monitors SSH connection and reconnects with exponential backoff
 * - Port persistence: saves forwarded ports to SharedPreferences, restores on Service restart
 * - WifiLock: prevents WiFi from disconnecting while SSH tunnel is active
 *
 * All SSH/HTTP network operations run on a background thread to avoid NetworkOnMainThreadException.
 */
public class TunnelEventService extends Service {

    private static final String TAG = "ClawBench";
    private static final int NOTIFICATION_ID = 2;
    private static final String CHANNEL_ID = "clawbench_ssh";
    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";
    private static final String KEY_SSH_PASSWORD = "ssh_password";
    private static final String KEY_SESSION_TOKEN = "session_token";
    private static final String KEY_FORWARDED_PORTS = "forwarded_ports";
    private static final String KEY_BATTERY_OPT_REQUESTED = "battery_opt_requested";

    // Reconnect parameters: exponential backoff delays in milliseconds
    private static final int[] RECONNECT_DELAYS_MS = {5000, 10000, 30000, 60000, 120000};
    private static final int MAX_RECONNECT_ATTEMPTS = 10;
    private static final int MONITOR_CHECK_INTERVAL_MS = 15000;

    private static volatile boolean isRunning = false;
    private static volatile TunnelEventService instance;

    private JSch jsch;
    private Session sshSession;
    private final Set<Integer> forwardedPorts = ConcurrentHashMap.newKeySet();
    private String serverHost;
    private int sshPort;
    private String password;
    private volatile String sessionToken; // Session token for ?token= auth (SSE, HTTP)

    // Background thread for all network I/O (SSH connect, HTTP fetch, port forward)
    private final ExecutorService networkExecutor = Executors.newSingleThreadExecutor();

    // SSE thread: receives system events for background notifications
    private Thread sseThread;
    private volatile boolean sseActive = false;

    // Whether the app is in the foreground (suppresses notifications)
    // Defaults to false — if service restarts before Activity.onCreate, notifications
    // should still be shown rather than silently dropped.
    private static volatile boolean appInForeground = false;

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
        Intent intent = new Intent(context, TunnelEventService.class);
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
        Intent intent = new Intent(context, TunnelEventService.class);
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
     * Save the session token for ?token= authentication.
     * Called from WebAppInterface.setSessionToken() after login.
     * The token is used for native SSE and HTTP requests (e.g. /api/ssh/info?token=xxx).
     */
    public static void setSessionToken(Context context, String token) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .edit()
                .putString(KEY_SESSION_TOKEN, token)
                .apply();
        // Update the running instance immediately
        // Capture to local to avoid TOCTOU race with onDestroy setting instance=null
        TunnelEventService localInstance = instance;
        if (localInstance != null) {
            localInstance.sessionToken = token;
            // Start SSE listener if not already running
            localInstance.startSSEListener();
        }
    }

    /**
     * Get the saved session token.
     */
    public static String getSessionToken(Context context) {
        return context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                .getString(KEY_SESSION_TOKEN, null);
    }

    /**
     * Set whether the app is in the foreground.
     * When in foreground, system notifications are suppressed (the WebView handles UI).
     * Called from MainActivity.onResume/onPause.
     */
    public static void setAppForeground(boolean foreground) {
        appInForeground = foreground;
    }

    /**
     * Ensure the service is running. Called after setSessionToken.
     */
    public static void ensureRunning(Context context) {
        if (!isRunning) {
            start(context);
        }
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

        // Load session token for SSE and authenticated HTTP requests
        sessionToken = getSessionToken(this);

        // Start SSE listener for system event notifications
        startSSEListener();

        // Note: We no longer stopSelf() when there are no forwarded ports.
        // The service stays alive for SSE notifications even without port forwards.
        // Service lifecycle is tied to login state, not port state.
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
            }
        } else {
            // START_STICKY restart: Android killed the service and recreated it with null intent.
            // onCreate() already restored port numbers into forwardedPorts via restoreForwardedPorts(),
            // but the SSH session was lost. Re-establish the tunnel now.
            if (!forwardedPorts.isEmpty()) {
                Log.i(TAG, "SSH: service restarted by START_STICKY, restoring " + forwardedPorts.size() + " port forwards");
                networkExecutor.execute(this::restoreAndReconnect);
            } else if (sessionToken != null && !sessionToken.isEmpty()) {
                // No ports but SSE needs the tunnel — connect SSH for SSE only
                Log.i(TAG, "SSH: service restarted by START_STICKY, connecting tunnel for SSE (no port forwards)");
                networkExecutor.execute(() -> {
                    try {
                        ensureConnection();
                    } catch (Exception e) {
                        Log.e(TAG, "SSH: failed to reconnect for SSE: " + e.getMessage());
                    }
                });
            }
        }

        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        intentionalDisconnect = true;
        stopSSEListener();
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
            Log.i(TAG, "SSH: cleaned up empty forwarded_ports from SharedPreferences");
        }

        stopForeground(true);
        super.onDestroy();
    }

    @Override
    public void onTaskRemoved(Intent rootIntent) {
        // When the user swipes the app from recents, restart the service
        // so the SSH tunnel keeps running.
        Intent restartIntent = new Intent(this, TunnelEventService.class);
        restartIntent.setAction("RESTORE_PORTS");
        PendingIntent pendingIntent = PendingIntent.getService(
                this, 1, restartIntent,
                PendingIntent.FLAG_ONE_SHOT | PendingIntent.FLAG_IMMUTABLE);
        AlarmManager alarmManager = (AlarmManager) getSystemService(Context.ALARM_SERVICE);
        if (alarmManager != null) {
            // Use setAndAllowWhileIdle() so the alarm fires even in Doze mode (Android 6+)
            alarmManager.setAndAllowWhileIdle(AlarmManager.ELAPSED_REALTIME_WAKEUP,
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
            Log.i(TAG, "SSH: restored " + forwardedPorts.size() + " forwarded ports from prefs");
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
                Log.i(TAG, "SSH: restored all port forwards after service restart");
            } catch (Exception e) {
                lastError = e.getMessage();
                Log.e(TAG, "SSH: failed to restore connection after service restart", e);
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
            Log.i(TAG, "SSH: connection monitor started");
            while (monitorActive && !Thread.currentThread().isInterrupted()) {
                try {
                    Thread.sleep(MONITOR_CHECK_INTERVAL_MS);
                } catch (InterruptedException e) {
                    break;
                }

                if (!monitorActive || intentionalDisconnect) break;

                // Re-acquire WakeLock periodically (it has a 1h timeout safety net)
                acquireWakeLock();

                // Check if session is dead
                if (sshSession == null || !sshSession.isConnected()) {
                    if (forwardedPorts.isEmpty()) {
                        // No ports to maintain — don't bother reconnecting
                        Log.d(TAG, "SSH: session disconnected but no ports to forward, skipping reconnect");
                        continue;
                    }

                    Log.w(TAG, "SSH: session disconnected, starting auto-reconnect");
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
                            Log.e(TAG, "SSH: exhausted " + MAX_RECONNECT_ATTEMPTS + " reconnect attempts, giving up");
                            updateNotification(forwardedPorts.size(), "SSH 隧道重连失败，请重新打开页面");
                            break;
                        }

                        try {
                            Log.i(TAG, "SSH: auto-reconnect attempt #" + reconnectAttempt);
                            ensureConnection();
                            lastError = null;
                            Log.i(TAG, "SSH: auto-reconnect succeeded on attempt #" + reconnectAttempt);
                            isReconnecting = false;
                            reconnectAttempt = 0;
                            updateNotification(forwardedPorts.size(), "SSH 隧道已恢复");

                            // Interrupt SSE thread so it reconnects through the new tunnel
                            if (sseThread != null && sseThread.isAlive()) {
                                Log.i(TAG, "SSE: interrupting listener to trigger reconnect via restored tunnel");
                                sseThread.interrupt();
                            }

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
                            Log.w(TAG, "SSH: auto-reconnect attempt #" + reconnectAttempt + " failed: " + e.getMessage());
                        }
                    }

                    if (isReconnecting) {
                        // Exhausted all attempts or monitor stopped
                        isReconnecting = false;
                    }
                }
            }
            Log.i(TAG, "SSH: connection monitor stopped");
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
                Log.i(TAG, "SSH: WifiLock acquired");
            }
        } catch (Exception e) {
            Log.w(TAG, "SSH: failed to acquire WifiLock", e);
        }
    }

    /**
     * Release the WifiLock.
     */
    private void releaseWifiLock() {
        if (wifiLock != null && wifiLock.isHeld()) {
            try {
                wifiLock.release();
                Log.i(TAG, "SSH: WifiLock released");
            } catch (Exception e) {
                Log.w(TAG, "SSH: failed to release WifiLock", e);
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

        Log.i(TAG, "SSH: connecting to " + serverHost + ":" + sshPort);

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

        Log.i(TAG, "SSH: connected to " + serverHost + ":" + sshPort);

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
                Log.i(TAG, "SSH: re-established port forward " + port);
            } catch (Exception e) {
                Log.e(TAG, "SSH: failed to re-establish port forward " + port, e);
            }
        }
        Log.i(TAG, "SSH: re-established " + reEstablished + "/" + forwardedPorts.size() + " port forwards");

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
            Log.d(TAG, "SSH: port " + port + " already forwarded and session active");
            return;
        }

        // Port is in the set but session is dead — need to reconnect.
        // Or port is new — need to add it.
        if (alreadyInSet && !sessionAlive) {
            Log.i(TAG, "SSH: port " + port + " in set but session disconnected, reconnecting...");
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
            Log.i(TAG, "SSH: port forward ready: localhost:" + port + " → server:" + port);
            updateNotification(forwardedPorts.size(), null);
        } catch (Exception e) {
            lastError = e.getMessage();
            Log.e(TAG, "SSH: failed to add port forward for " + port + ", retrying...", e);
            // Disconnect and retry once (password may have been updated, or session stale)
            disconnectInternal();
            try {
                ensureConnection();
                if (!alreadyInSet) {
                    sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
                    forwardedPorts.add(port);
                    saveForwardedPorts();
                }
                Log.i(TAG, "SSH: port forward added on retry: localhost:" + port + " → server:" + port);
                updateNotification(forwardedPorts.size(), null);
            } catch (Exception e2) {
                lastError = e2.getMessage();
                Log.e(TAG, "SSH: failed to add port forward for " + port + " on retry", e2);
            }
        }
    }

    /**
     * Remove a local port forward.
     */
    private synchronized void removePortForward(int port) {
        if (!forwardedPorts.contains(port)) {
            return;
        }

        try {
            if (sshSession != null && sshSession.isConnected()) {
                sshSession.delPortForwardingL(port);
                Log.i(TAG, "SSH: port forward removed: " + port);
            }
        } catch (Exception e) {
            Log.e(TAG, "SSH: failed to remove port forward for " + port, e);
        }

        forwardedPorts.remove(port);
        saveForwardedPorts();
        updateNotification(forwardedPorts.size(), null);

        // Note: We no longer stopSelf() when forwardedPorts is empty.
        // The service stays alive for SSE notifications even without port forwards.
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

            // Append ?token= for authentication (native HTTP client cannot use cookies)
            String token = sessionToken;
            if (token != null && !token.isEmpty()) {
                path += "?token=" + Uri.encode(token);
            }

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
                    Log.i(TAG, "SSH: fetched SSH port " + port + " from /api/ssh/info");
                    return port;
                } else {
                    Log.w(TAG, "SSH: SSH server is not enabled on the server");
                    return -1;
                }
            } else {
                Log.w(TAG, "SSH: /api/ssh/info returned HTTP " + code);
                return -1;
            }
        } catch (Exception e) {
            Log.w(TAG, "SSH: failed to fetch SSH info, will fallback to httpPort+1", e);
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
                Log.i(TAG, "SSH: disconnected");
            } catch (Exception e) {
                Log.e(TAG, "SSH: error during disconnect", e);
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
            android.app.NotificationChannel channel = new android.app.NotificationChannel(
                    CHANNEL_ID,
                    "SSH 端口转发",
                    android.app.NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription("SSH 隧道端口转发服务");
            android.app.NotificationManager nm = getSystemService(android.app.NotificationManager.class);
            if (nm != null) {
                nm.createNotificationChannel(channel);
            }
        }
    }

    /**
     * Build the foreground service notification.
     * @param portCount  Number of currently forwarded ports
     * @param statusText Optional status override (e.g. "SSH 隧道断开，正在重连…"). Null = normal status.
     */
    private Notification buildNotification(int portCount, String statusText) {
        Intent notificationIntent = new Intent(this, MainActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
                this, 0, notificationIntent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );

        String text;
        if (statusText != null) {
            text = statusText;
        } else if (portCount > 0) {
            text = portCount + " 个端口转发活跃";
        } else {
            // No ports forwarded — service should stop itself shortly,
            // but if this notification is visible it means we're winding down.
            text = "无端口转发，即将停止";
        }

        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle("ClawBench 端口转发")
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
     * Uses a 1-hour timeout as a safety net — the connection monitor re-acquires
     * periodically, and onDestroy releases explicitly.
     */
    private void acquireWakeLock() {
        if (wakeLock != null && wakeLock.isHeld()) return;
        try {
            PowerManager pm = (PowerManager) getApplicationContext().getSystemService(Context.POWER_SERVICE);
            if (pm != null) {
                wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "ClawBench:SSH-Tunnel");
                wakeLock.setReferenceCounted(false);
                wakeLock.acquire(3600_000L); // 1-hour timeout safety net
                Log.i(TAG, "SSH: WakeLock acquired (1h timeout)");
            }
        } catch (Exception e) {
            Log.w(TAG, "SSH: failed to acquire WakeLock", e);
        }
    }

    /**
     * Release the WakeLock.
     */
    private void releaseWakeLock() {
        if (wakeLock != null && wakeLock.isHeld()) {
            try {
                wakeLock.release();
                Log.i(TAG, "SSH: WakeLock released");
            } catch (Exception e) {
                Log.w(TAG, "SSH: failed to release WakeLock", e);
            }
            wakeLock = null;
        }
    }

    // --- Static helper methods for Activity to use ---

    /**
     * Add a port forward via the service.
     */
    public static void addForwardedPort(Context context, int port) {
        Intent intent = new Intent(context, TunnelEventService.class);
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
        Intent intent = new Intent(context, TunnelEventService.class);
        intent.setAction("REMOVE_PORT");
        intent.putExtra("port", port);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
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
            Log.e(TAG, "SSH: failed to init trust-all SSL context", e);
        }
    }

    // ───────────────────────────────────────────────────────────────────────
    // SSE listener for system event notifications
    // ───────────────────────────────────────────────────────────────────────

    /**
     * Start the SSE listener thread that connects to /api/events.
     * Receives session_complete and task_exec_update events to show system notifications
     * when the app is in the background.
     */
    private void startSSEListener() {
        if (sseActive) return; // Already running
        String token = sessionToken;
        if (token == null || token.isEmpty()) {
            Log.d(TAG, "SSE: no session token, skipping SSE listener");
            return;
        }

        sseActive = true;
        sseThread = new Thread(() -> {
            Log.i(TAG, "SSE: listener thread started");
            while (sseActive && !Thread.interrupted()) {
                HttpURLConnection conn = null;
                try {
                    // Prefer SSH tunnel: http://localhost:{httpPort} (single TCP connection, no extra keep-alive).
                    // Falls back to waiting if tunnel is not yet established.
                    boolean tunnelOk = isTunnelConnected();
                    if (!tunnelOk) {
                        Log.d(TAG, "SSE: SSH tunnel not connected, retrying in 5s");
                        Thread.sleep(5000);
                        continue;
                    }

                    String serverUrl = getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
                            .getString(KEY_SERVER_URL, null);
                    if (serverUrl == null) {
                        Log.w(TAG, "SSE: no server URL, retrying in 30s");
                        Thread.sleep(30000);
                        continue;
                    }

                    Uri uri = Uri.parse(serverUrl);
                    int httpPort = uri.getPort();
                    if (httpPort == -1) {
                        httpPort = serverUrl.startsWith("https://") ? 443 : 80;
                    }

                    // Always route through SSH tunnel: http://localhost:{httpPort}
                    // The tunnel maps localhost:{httpPort} on device → 127.0.0.1:{httpPort} on server.
                    // Server-side is plain HTTP (SSH already provides encryption).
                    String sseUrl = "http://localhost:" + httpPort + "/api/events?token=" + Uri.encode(sessionToken != null ? sessionToken : "");
                    Log.d(TAG, "SSE: connecting via SSH tunnel to " + sseUrl);
                    URL url = new URL(sseUrl);
                    conn = (HttpURLConnection) url.openConnection();
                    // No SSL needed — traffic goes through the encrypted SSH tunnel

                    conn.setRequestMethod("GET");
                    conn.setRequestProperty("Accept", "text/event-stream");
                    conn.setConnectTimeout(10000);
                    conn.setReadTimeout(60000); // Long timeout for SSE
                    conn.setDoInput(true);

                    int code = conn.getResponseCode();
                    if (code != 200) {
                        Log.w(TAG, "SSE: connection failed with HTTP " + code);
                        conn.disconnect();
                        Thread.sleep(10000);
                        continue;
                    }

                    Log.i(TAG, "SSE: connected to /api/events via SSH tunnel");
                    BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream()));
                    String currentEventType = null;

                    String line;
                    while (sseActive && (line = reader.readLine()) != null) {
                        if (line.startsWith("event:")) {
                            currentEventType = line.substring(6).trim();
                        } else if (line.startsWith("data:") && currentEventType != null) {
                            String data = line.substring(5).trim();
                            handleSSEEvent(currentEventType, data);
                            currentEventType = null;
                        } else if (line.isEmpty()) {
                            // End of SSE event — reset
                            currentEventType = null;
                        }
                    }
                    // Stream ended — will reconnect
                    reader.close();
                    conn.disconnect();
                    Log.w(TAG, "SSE: stream ended, reconnecting in 5s");
                    Thread.sleep(5000);
                } catch (InterruptedException e) {
                    // Interrupted — either intentional stop or tunnel-reconnect signal
                    if (!sseActive) {
                        Log.i(TAG, "SSE: listener thread interrupted, stopping");
                        break;
                    }
                    // sseActive still true → tunnel-reconnect signal, retry immediately
                    Log.i(TAG, "SSE: listener thread interrupted for reconnect, retrying");
                    continue;
                } catch (Exception e) {
                    Log.w(TAG, "SSE: connection error: " + e.getMessage());
                    if (conn != null) conn.disconnect();
                    try { Thread.sleep(10000); } catch (InterruptedException ie) {
                        if (!sseActive) break;
                        continue; // tunnel-reconnect signal
                    }
                }
            }
            sseActive = false;
            Log.i(TAG, "SSE: listener thread exited");
        }, "SSE-Listener");
        sseThread.setDaemon(true);
        sseThread.start();
    }

    /**
     * Stop the SSE listener thread.
     */
    private void stopSSEListener() {
        sseActive = false;
        if (sseThread != null) {
            sseThread.interrupt();
            sseThread = null;
        }
    }

    /**
     * Handle a received SSE event.
     * Only shows system notifications when the app is in the background.
     */
    private void handleSSEEvent(String eventType, String data) {
        Log.d(TAG, "SSE: event=" + eventType + " data=" + data);

        try {
            JSONObject json = new JSONObject(data);

            switch (eventType) {
                case "session_complete": {
                    if (!appInForeground) {
                        String sessionId = json.optString("sessionId", "");
                        String reason = json.optString("reason", "done");
                        String title = "ClawBench";
                        String text = reason.equals("user_cancel") ? "AI response cancelled" : "AI response completed";
                        showSystemNotification(title, text, "session_" + sessionId);
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
                        showSystemNotification(title, text, "task_" + taskId + "_" + status);
                    }
                    break;
                }
                case "connected": {
                    Log.i(TAG, "SSE: connected, clientId=" + json.optString("clientId", "unknown"));
                    break;
                }
            }
        } catch (Exception e) {
            Log.w(TAG, "SSE: failed to parse event data", e);
        }
    }

    /**
     * Show a system notification (only when app is in the background).
     */
    private void showSystemNotification(String title, String text, String tag) {
        NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);

        // Create a separate channel for event notifications
        String eventChannelId = "clawbench_events";
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    eventChannelId, "Event Notifications", NotificationManager.IMPORTANCE_DEFAULT);
            channel.setDescription("AI completion and task notifications");
            nm.createNotificationChannel(channel);
        }

        Intent intent = new Intent(this, MainActivity.class);
        intent.setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        PendingIntent pendingIntent = PendingIntent.getActivity(this, 0, intent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);

        Notification notification = new NotificationCompat.Builder(this, eventChannelId)
                .setSmallIcon(android.R.drawable.ic_dialog_info)
                .setContentTitle(title)
                .setContentText(text)
                .setAutoCancel(true)
                .setContentIntent(pendingIntent)
                .setGroup("clawbench_events")
                .build();

        // Use a unique ID based on the tag hash to avoid overwriting
        int id = (tag != null ? tag.hashCode() : 0) & 0xFFFF;
        nm.notify(id, notification);
    }
}
