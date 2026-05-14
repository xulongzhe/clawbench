package com.clawbench.app;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.app.DownloadManager;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.ConnectivityManager;
import android.net.NetworkInfo;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.PowerManager;
import android.provider.MediaStore;
import android.provider.Settings;
import android.util.Log;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.widget.FrameLayout;
import android.webkit.DownloadListener;
import android.webkit.JavascriptInterface;
import android.webkit.SslErrorHandler;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.ProgressBar;
import android.widget.Toast;

import android.content.pm.PackageManager;
import android.Manifest;

import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.appcompat.app.AppCompatActivity;
import androidx.core.content.ContextCompat;

import org.json.JSONArray;

import java.io.File;
import java.io.IOException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;
import java.util.Set;

/**
 * Main Activity: hosts a fullscreen WebView that connects to the ClawBench server.
 *
 * Key features:
 * - Server URL configuration dialog on first launch
 * - WebView with JS, DOM storage, and media autoplay enabled
 * - JavaScript interface for native bridge (port forwarding, SSH password)
 * - Port forwarding via SSH tunnels (PortForwardService) — transparent localhost access
 * - Proper back navigation within WebView
 * - SSL error handling with user confirmation
 */
public class MainActivity extends AppCompatActivity {

    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";
    private static final String KEY_SSH_PASSWORD = "ssh_password";
    private static final String TAG = "ClawBench";

    private WebView webView;
    private ProgressBar progressBar;
    private SharedPreferences prefs;

    // File chooser state for WebView <input type="file"> support
    private ValueCallback<Uri[]> filePathCallback;
    private Uri cameraImageUri; // URI for camera capture image

    // Notification permission launcher (Android 13+) — required for foreground service notification
    private final ActivityResultLauncher<String> notificationPermissionLauncher =
            registerForActivityResult(new ActivityResultContracts.RequestPermission(), isGranted -> {
                if (!isGranted) {
                    Log.w(TAG, "POST_NOTIFICATIONS permission denied — tunnel notification will not show");
                }
            });

    // Activity result launcher for file chooser (replaces deprecated onActivityResult)
    private final ActivityResultLauncher<Intent> fileChooserLauncher =
            registerForActivityResult(new ActivityResultContracts.StartActivityForResult(), result -> {
                if (filePathCallback == null) return;
                Uri[] results = null;
                if (result.getResultCode() == Activity.RESULT_OK && result.getData() != null) {
                    Intent data = result.getData();
                    String dataString = data.getDataString();
                    if (dataString != null) {
                        results = new Uri[]{ Uri.parse(dataString) };
                    } else if (data.getClipData() != null) {
                        // Multiple files selected
                        int count = data.getClipData().getItemCount();
                        results = new Uri[count];
                        for (int i = 0; i < count; i++) {
                            results[i] = data.getClipData().getItemAt(i).getUri();
                        }
                    }
                    // Only use camera URI as fallback when RESULT_OK — this means
                    // the user actively chose the camera option and took a photo.
                    // Without this guard, cancelling the picker would falsely report
                    // the pre-created camera temp file as the "selected" file.
                    if (results == null && cameraImageUri != null) {
                        results = new Uri[]{ cameraImageUri };
                    }
                }
                // Use empty array instead of null for cancellation — some WebView implementations
                // fire the JS change event with stale file data when onReceiveValue(null) is called.
                // An empty array explicitly means "0 files selected".
                filePathCallback.onReceiveValue(results != null ? results : new Uri[0]);
                filePathCallback = null;
                // Clean up unused camera temp file if user didn't take a photo
                if (cameraImageUri != null && (results == null || !cameraImageUri.equals(results[0]))) {
                    new File(cameraImageUri.getPath()).delete();
                }
                cameraImageUri = null;
            });

    // Set of ports currently being forwarded (thread-safe for access from WebView background threads)
    final Set<Integer> forwardedPorts = java.util.concurrent.ConcurrentHashMap.newKeySet();

    // Volume key interception mode: when true, volume up/down are forwarded to WebView
    // as JS calls instead of adjusting system volume. Controlled by the terminal panel.
    private volatile boolean volumeKeyMode = false;

    // Fullscreen video state: managed by WebChromeClient.onShowCustomView/onHideCustomView
    private View customView;
    private WebChromeClient.CustomViewCallback customViewCallback;
    private int originalOrientation;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Keep screen on while app is in foreground (AI may take time to respond)
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);

        // Initialize trust-all SSL for self-signed HTTPS servers (used by PortForwardService)
        PortForwardService.initTrustAllSSL();

        setContentView(R.layout.activity_main);

        webView = findViewById(R.id.webView);
        progressBar = findViewById(R.id.progressBar);

        prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);

        // Request notification permission (Android 13+) — required for foreground service notification
        requestNotificationPermission();

        // Auto-restore PortForwardService if there are previously saved ports.
        // This ensures the SSH tunnel and its notification are active immediately on cold start,
        // without waiting for the WebView to load and syncToNative() to fire.
        restorePortForwardServiceIfNeeded();

        setupWebView();

        // Load saved URL or show configuration dialog
        String savedUrl = prefs.getString(KEY_SERVER_URL, null);
        if (savedUrl != null) {
            loadUrl(savedUrl);
        } else {
            showServerDialog();
        }
    }

    @SuppressLint("SetJavaScriptEnabled")
    private void setupWebView() {
        WebSettings settings = webView.getSettings();

        // Core web features
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);
        settings.setCacheMode(WebSettings.LOAD_DEFAULT);

        // Critical: allow audio.play() without user gesture (enables TTS in lock screen)
        settings.setMediaPlaybackRequiresUserGesture(false);

        // Allow mixed content (HTTP API calls from HTTPS page, or vice versa)
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);

        // File access for uploads
        settings.setAllowFileAccess(true);
        settings.setAllowContentAccess(true);

        // Responsive layout
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(true);

        // Enable smooth scrolling
        webView.setOverScrollMode(WebView.OVER_SCROLL_NEVER);

        // Set custom user agent
        String ua = settings.getUserAgentString();
        settings.setUserAgentString(ua + " ClawBench-Android/1.0");

        // JavaScript interface for native bridge
        webView.addJavascriptInterface(new WebAppInterface(this), "AndroidNative");

        // WebView client for navigation and error handling
        webView.setWebViewClient(new ClawBenchWebViewClient());

        // Chrome client for progress and file chooser
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onProgressChanged(WebView view, int newProgress) {
                if (newProgress < 100) {
                    progressBar.setVisibility(View.VISIBLE);
                } else {
                    progressBar.setVisibility(View.GONE);
                }
            }

            @Override
            public boolean onShowFileChooser(WebView webView, ValueCallback<Uri[]> callback, FileChooserParams fileChooserParams) {
                // Cancel any previous pending request
                if (filePathCallback != null) {
                    filePathCallback.onReceiveValue(null);
                }
                filePathCallback = callback;

                // Build the file chooser intent from the <input> element's accept/multiple attributes
                Intent chooserIntent;
                try {
                    chooserIntent = fileChooserParams.createIntent();
                } catch (Exception e) {
                    // Fallback: generic file picker
                    chooserIntent = new Intent(Intent.ACTION_GET_CONTENT);
                    chooserIntent.addCategory(Intent.CATEGORY_OPENABLE);
                    chooserIntent.setType("*/*");
                    if (fileChooserParams.getMode() == FileChooserParams.MODE_OPEN_MULTIPLE) {
                        chooserIntent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true);
                    }
                }

                // Offer camera as an additional option
                Intent cameraIntent = null;
                try {
                    cameraIntent = new Intent(MediaStore.ACTION_IMAGE_CAPTURE);
                    if (cameraIntent.resolveActivity(getPackageManager()) != null) {
                        File photoFile = createImageFile();
                        if (photoFile != null) {
                            cameraImageUri = androidx.core.content.FileProvider.getUriForFile(
                                    MainActivity.this,
                                    getPackageName() + ".fileprovider",
                                    photoFile
                            );
                            cameraIntent.putExtra(MediaStore.EXTRA_OUTPUT, cameraImageUri);
                        } else {
                            cameraIntent = null;
                        }
                    } else {
                        cameraIntent = null;
                    }
                } catch (Exception e) {
                    Log.w(TAG, "Camera intent not available", e);
                    cameraIntent = null;
                }

                try {
                    if (cameraIntent != null) {
                        // Show chooser with both file picker and camera options
                        chooserIntent = Intent.createChooser(chooserIntent, "选择文件");
                        chooserIntent.putExtra(Intent.EXTRA_INITIAL_INTENTS, new Intent[]{ cameraIntent });
                    }
                    fileChooserLauncher.launch(chooserIntent);
                } catch (Exception e) {
                    Log.e(TAG, "File chooser failed to launch", e);
                    filePathCallback = null;
                    return false;
                }
                return true;
            }

            @Override
            public void onShowCustomView(View view, CustomViewCallback callback) {
                // If already in fullscreen, dismiss the previous one first
                if (customView != null) {
                    onHideCustomView();
                    return;
                }
                customView = view;
                customViewCallback = callback;

                // Save current orientation and switch to landscape for video
                originalOrientation = getRequestedOrientation();
                setRequestedOrientation(android.content.pm.ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE);

                // Hide the WebView and show the custom fullscreen view
                webView.setVisibility(View.GONE);
                FrameLayout container = findViewById(R.id.webView).getParent() instanceof FrameLayout
                        ? (FrameLayout) findViewById(R.id.webView).getParent() : null;
                if (container != null) {
                    container.addView(view, new FrameLayout.LayoutParams(
                            FrameLayout.LayoutParams.MATCH_PARENT,
                            FrameLayout.LayoutParams.MATCH_PARENT));
                }

                // Hide system UI for immersive fullscreen
                getWindow().getDecorView().setSystemUiVisibility(
                        View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                        | View.SYSTEM_UI_FLAG_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
            }

            @Override
            public void onHideCustomView() {
                if (customView == null) return;

                // Remove the custom view
                FrameLayout container = customView.getParent() instanceof FrameLayout
                        ? (FrameLayout) customView.getParent() : null;
                if (container != null) {
                    container.removeView(customView);
                }
                customView = null;

                // Restore orientation
                setRequestedOrientation(originalOrientation);

                // Show the WebView again
                webView.setVisibility(View.VISIBLE);

                if (customViewCallback != null) {
                    customViewCallback.onCustomViewHidden();
                    customViewCallback = null;
                }

                // Restore system UI
                getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_VISIBLE);
            }
        });

        // Enable cookies (needed for auth session)
        CookieManager.getInstance().setAcceptThirdPartyCookies(webView, true);

        // Handle file downloads via DownloadManager
        webView.setDownloadListener((url, userAgent, contentDisposition, mimetype, contentLength) -> {
            try {
                DownloadManager.Request request = new DownloadManager.Request(Uri.parse(url));
                // Carry auth cookies so the download is authorized
                String cookies = CookieManager.getInstance().getCookie(url);
                if (cookies != null) {
                    request.addRequestHeader("Cookie", cookies);
                }
                request.setMimeType(mimetype);
                request.setTitle(getFileNameFromUrl(url));
                request.setDescription(getString(R.string.download_description));
                request.allowScanningByMediaScanner();
                request.setNotificationVisibility(
                        DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
                request.setDestinationInExternalPublicDir(
                        Environment.DIRECTORY_DOWNLOADS, "ClawBench/" + getFileNameFromUrl(url));

                DownloadManager dm = (DownloadManager) getSystemService(Context.DOWNLOAD_SERVICE);
                dm.enqueue(request);
                Toast.makeText(this, R.string.download_started, Toast.LENGTH_SHORT).show();
            } catch (Exception e) {
                Log.e(TAG, "Download failed", e);
                Toast.makeText(this, R.string.download_failed, Toast.LENGTH_SHORT).show();
            }
        });
    }

    /**
     * Extract a file name from a /api/local-file/ URL.
     * Falls back to "download" if parsing fails.
     */
    private String getFileNameFromUrl(String url) {
        String decoded = Uri.decode(url);
        int lastSlash = decoded.lastIndexOf('/');
        if (lastSlash >= 0 && lastSlash < decoded.length() - 1) {
            return decoded.substring(lastSlash + 1);
        }
        return "download";
    }

    /**
     * Create a temporary image file for camera capture.
     * Used by onShowFileChooser to provide a URI for the camera intent.
     */
    private File createImageFile() {
        String timestamp = new SimpleDateFormat("yyyyMMdd_HHmmss", Locale.US).format(new Date());
        File storageDir = getExternalFilesDir(Environment.DIRECTORY_PICTURES);
        try {
            return File.createTempFile("IMG_" + timestamp, ".jpg", storageDir);
        } catch (IOException e) {
            Log.e(TAG, "Failed to create image file", e);
            return null;
        }
    }

    private void loadUrl(String url) {
        if (isNetworkAvailable()) {
            webView.loadUrl(url);
        } else {
            Toast.makeText(this, R.string.error_connection_failed, Toast.LENGTH_LONG).show();
        }
    }

    @SuppressWarnings("deprecation")
    private boolean isNetworkAvailable() {
        ConnectivityManager cm = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
        if (cm == null) return false;
        NetworkInfo ni = cm.getActiveNetworkInfo();
        return ni != null && ni.isConnected();
    }

    /**
     * Request POST_NOTIFICATIONS runtime permission on Android 13+ (API 33+).
     * Required for the PortForwardService foreground notification to be visible.
     */
    private void requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                    != PackageManager.PERMISSION_GRANTED) {
                notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS);
            }
        }
    }

    /**
     * Show dialog for user to input ClawBench server URL.
     * Called on first launch or when user wants to change server.
     */
    private void showServerDialog() {
        View dialogView = getLayoutInflater().inflate(R.layout.dialog_server_url, null);
        EditText input = dialogView.findViewById(R.id.serverUrlInput);
        EditText passwordInput = dialogView.findViewById(R.id.passwordInput);

        // Pre-fill with saved URL if exists
        String savedUrl = prefs.getString(KEY_SERVER_URL, "");
        input.setText(savedUrl);
        input.setSelection(input.getText().length());

        // Pre-fill with saved password if exists
        String savedPassword = prefs.getString(KEY_SSH_PASSWORD, "");
        passwordInput.setText(savedPassword);

        new AlertDialog.Builder(this)
                .setTitle(R.string.dialog_title)
                .setView(dialogView)
                .setPositiveButton(R.string.dialog_positive, (dialog, which) -> {
                    String url = input.getText().toString().trim();
                    if (url.isEmpty()) {
                        Toast.makeText(this, R.string.error_no_url, Toast.LENGTH_SHORT).show();
                        showServerDialog();
                        return;
                    }
                    if (!url.startsWith("http://") && !url.startsWith("https://")) {
                        Toast.makeText(this, R.string.error_invalid_url, Toast.LENGTH_SHORT).show();
                        showServerDialog();
                        return;
                    }
                    // Remove trailing slash
                    if (url.endsWith("/")) {
                        url = url.substring(0, url.length() - 1);
                    }
                    prefs.edit().putString(KEY_SERVER_URL, url).apply();

                    // Save password for SSH tunnel
                    String pwd = passwordInput.getText().toString();
                    if (!pwd.isEmpty()) {
                        PortForwardService.setPassword(this, pwd);
                    }

                    loadUrl(url);
                })
                .setNegativeButton(R.string.dialog_negative, null)
                .setCancelable(false)
                .show();
    }

    /**
     * If there are previously saved forwarded ports in SharedPreferences,
     * start the PortForwardService immediately so the SSH tunnel and its
     * notification are active on cold start.
     * This avoids the gap where no notification shows until the WebView
     * finishes loading and syncToNative() fires.
     */
    private void restorePortForwardServiceIfNeeded() {
        Set<String> savedPorts = prefs.getStringSet("forwarded_ports", null);
        if (savedPorts != null && !savedPorts.isEmpty()) {
            Log.i(TAG, "Cold start: restoring PortForwardService with " + savedPorts.size() + " saved ports");
            PortForwardService.start(this);
        }
    }

    @Override
    public void onBackPressed() {
        // If in fullscreen video mode, exit fullscreen first
        if (customView != null) {
            WebChromeClient client = webView.getWebChromeClient();
            if (client != null) {
                client.onHideCustomView();
            }
            return;
        }
        if (webView.canGoBack()) {
            webView.goBack();
        } else {
            // No more WebView history — just exit the app.
            // Server reconfiguration is available via the gear menu in the web UI
            // when the page fails to load, rather than through a back-gesture dialog.
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        // Do NOT stop PortForwardService here — it should survive Activity lifecycle
        // so the SSH tunnel continues running when the app is in background.
        super.onDestroy();
    }

    /**
     * Intercept volume key events when volumeKeyMode is enabled (terminal panel open).
     * Instead of adjusting system volume, dispatch them to the WebView as JS callbacks.
     * All other keys fall through to the default handling.
     */
    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        if (volumeKeyMode) {
            int keyCode = event.getKeyCode();
            if (keyCode == KeyEvent.KEYCODE_VOLUME_UP || keyCode == KeyEvent.KEYCODE_VOLUME_DOWN) {
                // Only act on ACTION_DOWN to avoid double-firing on ACTION_UP
                if (event.getAction() == KeyEvent.ACTION_DOWN) {
                    String direction = keyCode == KeyEvent.KEYCODE_VOLUME_UP ? "up" : "down";
                    webView.evaluateJavascript(
                            "if(typeof __onVolumeKey==='function'){__onVolumeKey('" + direction + "')}", null);
                }
                return true; // consume the event — no system volume change
            }
        }
        return super.dispatchKeyEvent(event);
    }

    // --- WebView Client ---

    private class ClawBenchWebViewClient extends WebViewClient {

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            Uri url = request.getUrl();
            String host = url.getHost();

            // Allow localhost and the configured server
            String serverUrl = prefs.getString(KEY_SERVER_URL, "");
            String serverHost = Uri.parse(serverUrl).getHost();

            if ("localhost".equals(host) || "127.0.0.1".equals(host) || host.equals(serverHost)) {
                return false; // Load in WebView
            }

            // Open external links in system browser
            Intent intent = new Intent(Intent.ACTION_VIEW, url);
            startActivity(intent);
            return true;
        }

        @Override
        public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
            String url = view.getUrl();
            // Auto-accept SSL errors for localhost (SSH tunnel forwards — cert won't match localhost)
            if (url != null && (url.startsWith("https://localhost:") || url.startsWith("https://127.0.0.1:"))) {
                handler.proceed();
                return;
            }
            String serverUrl = prefs.getString(KEY_SERVER_URL, "");
            if (serverUrl.startsWith("https://")) {
                new AlertDialog.Builder(MainActivity.this)
                        .setTitle("SSL 证书验证失败")
                        .setMessage("服务器使用了自签名证书，连接可能不安全。\n\n仅当您信任该服务器时才继续。")
                        .setPositiveButton("信任并继续", (dialog, which) -> handler.proceed())
                        .setNegativeButton("取消连接", (dialog, which) -> handler.cancel())
                        .setCancelable(false)
                        .show();
            } else {
                handler.cancel();
            }
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            super.onReceivedError(view, request, error);
            // Only handle main frame errors — offer reconfigure when the page itself fails to load.
            // Defer the check: if the connection recovers before the deferred runnable fires,
            // we avoid showing a stale dialog after screen unlock.
            if (request.isForMainFrame()) {
                view.postDelayed(() -> {
                    if (!isFinishing() && !isDestroyed()) {
                        showLoadErrorDialog();
                    }
                }, 600);
            }
        }

        /**
         * Show dialog when the configured server URL cannot be reached.
         * Offers to reconfigure the server or retry.
         */
        private void showLoadErrorDialog() {
            new AlertDialog.Builder(MainActivity.this)
                    .setTitle(R.string.error_connection_failed)
                    .setMessage("无法连接到服务器，请检查服务器地址和网络连接。")
                    .setPositiveButton("重新配置", (dialog, which) -> showServerDialog())
                    .setNegativeButton("重试", (dialog, which) -> {
                        String url = prefs.getString(KEY_SERVER_URL, "");
                        if (!url.isEmpty()) loadUrl(url);
                    })
                    .setCancelable(true)
                    .show();
        }
    }

    // --- JavaScript Interface ---

    public static class WebAppInterface {
        private final MainActivity activity;

        public WebAppInterface(MainActivity activity) {
            this.activity = activity;
        }

        @JavascriptInterface
        public String getAppVersion() {
            try {
                return activity.getPackageManager()
                        .getPackageInfo(activity.getPackageName(), 0).versionName;
            } catch (Exception e) {
                return "1.0.0";
            }
        }

        @JavascriptInterface
        public boolean isNativeApp() {
            return true;
        }

        /**
         * Check whether the SSH tunnel is currently connected.
         * Queries the PortForwardService's SSH session state.
         * Returns true if connected, false if disconnected or service not running.
         */
        @JavascriptInterface
        public boolean isTunnelConnected() {
            return PortForwardService.isTunnelConnected();
        }

        /**
         * Get the last SSH connection error message.
         * Returns empty string if no error, or a descriptive error message.
         * Used by the frontend to show specific failure reasons
         * (auth failure, network unreachable, etc.) in the tunnel status banner.
         */
        @JavascriptInterface
        public String getTunnelError() {
            String err = PortForwardService.getLastError();
            return err != null ? err : "";
        }

        /**
         * Get the type of the last SSH connection error.
         * Returns one of: "auth", "network", "hostkey", "unknown", or empty string if no error.
         * Used by the frontend to show localized error messages.
         */
        @JavascriptInterface
        public String getTunnelErrorType() {
            String type = PortForwardService.getErrorType();
            return type != null ? type : "";
        }

        /**
         * Add a port to be forwarded via SSH tunnel.
         * The PortForwardService creates a local port forward: localhost:{port} → server:{port}
         * WebView can then access http://localhost:{port} directly.
         * Also requests battery optimization exemption on first port forward.
         */
        @JavascriptInterface
        public void addForwardedPort(int port) {
            activity.runOnUiThread(() -> {
                activity.forwardedPorts.add(port);
                PortForwardService.addForwardedPort(activity, port);

                // Request battery optimization exemption if not already granted.
                // Re-check every time in case the user previously dismissed the dialog.
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                    PowerManager pm = (PowerManager) activity.getSystemService(Context.POWER_SERVICE);
                    if (pm != null && !pm.isIgnoringBatteryOptimizations(activity.getPackageName())) {
                        requestIgnoreBatteryOptimization();
                    }
                }
            });
        }

        /**
         * Request the system to exclude ClawBench from battery optimization.
         * This prevents the OS from aggressively killing the port forward service.
         * Only requested once — tracked via SharedPreferences.
         */
        private void requestIgnoreBatteryOptimization() {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                PowerManager pm = (PowerManager) activity.getSystemService(Context.POWER_SERVICE);
                String packageName = activity.getPackageName();
                if (pm != null && !pm.isIgnoringBatteryOptimizations(packageName)) {
                    try {
                        Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                        intent.setData(Uri.parse("urn:android:pkg:" + packageName));
                        activity.startActivity(intent);
                        PortForwardService.setBatteryOptRequested(activity);
                        Log.i(TAG, "Requested battery optimization exemption");
                    } catch (Exception e) {
                        Log.w(TAG, "Failed to request battery optimization exemption", e);
                    }
                } else {
                    // Already whitelisted, just mark as requested
                    PortForwardService.setBatteryOptRequested(activity);
                }
            }
        }

        /**
         * Remove a port forward.
         */
        @JavascriptInterface
        public void removeForwardedPort(int port) {
            activity.runOnUiThread(() -> {
                activity.forwardedPorts.remove(port);
                PortForwardService.removeForwardedPort(activity, port);
            });
        }

        /**
         * Stop the PortForwardService and disconnect SSH.
         * Called from WebView when server reports no forwarded ports,
         * to avoid running an idle foreground service with no work to do.
         */
        @JavascriptInterface
        public void stopPortForwardService() {
            activity.runOnUiThread(() -> {
                Log.i(TAG, "WebView requested PortForwardService stop (no ports on server)");
                activity.forwardedPorts.clear();
                PortForwardService.stop(activity);
            });
        }

        @JavascriptInterface
        public String getForwardedPorts() {
            return new JSONArray(activity.forwardedPorts).toString();
        }

        @JavascriptInterface
        public String getServerUrl() {
            return activity.prefs.getString(KEY_SERVER_URL, "");
        }

        /**
         * Show the server configuration dialog (change URL/password).
         * Called from WebView when connection fails or user wants to reconfigure.
         */
        @JavascriptInterface
        public void showServerDialog() {
            activity.runOnUiThread(() -> activity.showServerDialog());
        }

        /**
         * Open a forwarded port in the system browser.
         * Called from the port forwarding panel "open" button.
         */
        @JavascriptInterface
        public void openInBrowser(int port, String protocol) {
            activity.runOnUiThread(() -> {
                String scheme = "https".equalsIgnoreCase(protocol) ? "https" : "http";
                String url = scheme + "://localhost:" + port;
                Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(url));
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                activity.startActivity(intent);
            });
        }

        /**
         * Open a forwarded port in the sandbox browser (BrowserActivity).
         * Runs in a separate process for full Cookie/Storage isolation from the main app.
         * Called from the port forwarding panel "open" button (preferred over openInBrowser).
         */
        @JavascriptInterface
        public void openInSandbox(int port, String protocol) {
            activity.runOnUiThread(() -> {
                String scheme = "https".equalsIgnoreCase(protocol) ? "https" : "http";
                Intent intent = new Intent(activity, BrowserActivity.class);
                intent.putExtra("port", port);
                intent.putExtra("protocol", scheme);
                activity.startActivity(intent);
            });
        }

        /**
         * Get the saved SSH/web password for auto-login.
         * Returns empty string if no password is saved.
         */
        @JavascriptInterface
        public String getPassword() {
            return activity.prefs.getString(KEY_SSH_PASSWORD, "");
        }

        /**
         * Save the SSH password. Called from WebView after successful login.
         * The same password is used for both web auth and SSH auth.
         */
        @JavascriptInterface
        public void setSSHPassword(String pwd) {
            PortForwardService.setPassword(activity, pwd);
        }

        /**
         * Download a file from the ClawBench server to the Downloads directory.
         * @param path Relative file path (as used in /api/local-file/ URL)
         */
        @JavascriptInterface
        public void downloadFile(String path) {
            activity.runOnUiThread(() -> {
                String serverUrl = activity.prefs.getString(KEY_SERVER_URL, "");
                if (serverUrl.isEmpty()) return;
                String url = serverUrl + "/api/local-file/" + Uri.encode(path, "/") + "?download=1";
                // Trigger the DownloadListener by asking WebView to load the URL
                // The ?download=1 param makes the server return Content-Disposition: attachment
                // which forces WebView to trigger the DownloadListener instead of rendering inline
                activity.webView.loadUrl(url);
            });
        }

        /**
         * Enable or disable volume key interception mode.
         * When enabled, volume up/down keys are forwarded to the WebView JS layer
         * via __onVolumeKey() callback instead of adjusting system volume.
         * Called by the terminal panel on open/close.
         * @param enabled true to intercept volume keys, false to restore default behavior
         */
        @JavascriptInterface
        public void setVolumeKeyMode(boolean enabled) {
            activity.volumeKeyMode = enabled;
        }
    }
}
