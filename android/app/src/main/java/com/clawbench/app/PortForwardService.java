package com.clawbench.app;

import android.app.Notification;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.IBinder;
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
public class PortForwardService extends Service {

    private static final String TAG = "ClawBench";
    private static final int NOTIFICATION_ID = 2;
    private static final String CHANNEL_ID = "clawbench_ssh";
    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";
    private static final String KEY_SSH_PASSWORD = "ssh_password";
    private static final String KEY_FORWARDED_PORTS = "forwarded_ports";
    private static final String KEY_BATTERY_OPT_REQUESTED = "battery_opt_requested";

    // Reconnect parameters: exponential backoff delays in milliseconds
    private static final int[] RECONNECT_DELAYS_MS = {5000, 10000, 30000, 60000, 120000};
    private static final int MONITOR_CHECK_INTERVAL_MS = 15000;

    private static boolean isRunning = false;

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

    // Reconnect state
    private volatile boolean isReconnecting = false;
    private volatile int reconnectAttempt = 0;

    public static boolean isRunning() {
        return isRunning;
    }

    /**
     * Start the port forward service.
     */
    public static void start(Context context) {
        if (isRunning) return;
        Intent intent = new Intent(context, PortForwardService.class);
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
        Intent intent = new Intent(context, PortForwardService.class);
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
        jsch = new JSch();
        startForeground(NOTIFICATION_ID, buildNotification(0, null));

        // Restore previously saved ports (from before Service was killed)
        restoreForwardedPorts();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (!isRunning) {
            isRunning = true;
            startForeground(NOTIFICATION_ID, buildNotification(0, null));
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
                // Service restarted by START_STICKY — restore ports and reconnect
                networkExecutor.execute(this::restoreAndReconnect);
            }
        }

        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        intentionalDisconnect = true;
        stopConnectionMonitor();
        releaseWifiLock();
        disconnectInternal();
        isRunning = false;
        networkExecutor.shutdownNow();
        stopForeground(true);
        super.onDestroy();
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
                                    "SSH 隧道断开，第 " + reconnectAttempt + " 次重连…");
                            try {
                                Thread.sleep(delay);
                            } catch (InterruptedException e) {
                                break;
                            }
                        }

                        if (!monitorActive || intentionalDisconnect) break;

                        try {
                            Log.i(TAG, "SSH: auto-reconnect attempt #" + reconnectAttempt);
                            ensureConnection();
                            Log.i(TAG, "SSH: auto-reconnect succeeded on attempt #" + reconnectAttempt);
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
            sshPort = httpPort + 1; // fallback: HTTP port + 1
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

        Log.i(TAG, "SSH: connected to " + serverHost + ":" + sshPort);

        // Acquire WifiLock to prevent WiFi from being disabled
        acquireWifiLock();

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
     * If the connection fails, disconnects and retries once (password may have been updated).
     * MUST be called from a background thread (network I/O).
     */
    private synchronized void addPortForward(int port) {
        if (forwardedPorts.contains(port)) {
            Log.d(TAG, "SSH: port " + port + " already forwarded");
            return;
        }

        try {
            ensureConnection();
            sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
            forwardedPorts.add(port);
            saveForwardedPorts();
            Log.i(TAG, "SSH: port forward added: localhost:" + port + " → server:" + port);
            updateNotification(forwardedPorts.size(), null);
        } catch (Exception e) {
            Log.e(TAG, "SSH: failed to add port forward for " + port + ", retrying...", e);
            // Disconnect and retry once (password may have been updated, or session stale)
            disconnectInternal();
            try {
                ensureConnection();
                sshSession.setPortForwardingL("127.0.0.1", port, "127.0.0.1", port);
                forwardedPorts.add(port);
                saveForwardedPorts();
                Log.i(TAG, "SSH: port forward added on retry: localhost:" + port + " → server:" + port);
                updateNotification(forwardedPorts.size(), null);
            } catch (Exception e2) {
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

        // If no more forwarded ports, stop the service
        if (forwardedPorts.isEmpty()) {
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
            text = "SSH 隧道已连接";
        }

        // Create notification channel for Android O+
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

        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle("ClawBench 端口转发")
                .setContentText(text)
                .setSmallIcon(R.drawable.ic_notification)
                .setContentIntent(pendingIntent)
                .setOngoing(true)
                .setSilent(true)
                .build();
    }

    // --- Static helper methods for Activity to use ---

    /**
     * Add a port forward via the service.
     */
    public static void addForwardedPort(Context context, int port) {
        Intent intent = new Intent(context, PortForwardService.class);
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
        Intent intent = new Intent(context, PortForwardService.class);
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
            Log.e(TAG, "SSH: failed to init trust-all SSL context", e);
        }
    }
}
