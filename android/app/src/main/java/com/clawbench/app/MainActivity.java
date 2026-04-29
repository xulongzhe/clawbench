package com.clawbench.app;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.ConnectivityManager;
import android.net.NetworkInfo;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Build;
import android.os.Bundle;
import android.view.Menu;
import android.view.MenuItem;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.JavascriptInterface;
import android.webkit.SslErrorHandler;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.ProgressBar;
import android.widget.Toast;

import androidx.swiperefreshlayout.widget.SwipeRefreshLayout;

/**
 * Main Activity: hosts a fullscreen WebView that connects to the ClawBench server.
 *
 * Key features:
 * - Server URL configuration dialog on first launch
 * - WebView with JS, DOM storage, and media autoplay enabled
 * - JavaScript interface for native bridge (keep-alive service control)
 * - Swipe-down refresh + overflow menu (refresh, switch server, clear cache)
 * - Proper back navigation within WebView
 * - SSL error handling with user confirmation
 */
public class MainActivity extends Activity {

    private static final String PREFS_NAME = "clawbench_prefs";
    private static final String KEY_SERVER_URL = "server_url";
    private static final String CHANNEL_ID = "clawbench_chat";

    private WebView webView;
    private ProgressBar progressBar;
    private SwipeRefreshLayout swipeRefresh;
    private SharedPreferences prefs;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Keep screen on while app is in foreground (AI may take time to respond)
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);

        setContentView(R.layout.activity_main);

        webView = findViewById(R.id.webView);
        progressBar = findViewById(R.id.progressBar);
        swipeRefresh = findViewById(R.id.swipeRefresh);

        prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);

        createNotificationChannel();
        setupSwipeRefresh();
        setupWebView();

        // Load saved URL or show configuration dialog
        String savedUrl = prefs.getString(KEY_SERVER_URL, null);
        if (savedUrl != null) {
            loadUrl(savedUrl);
        } else {
            showServerDialog();
        }
    }

    private void setupSwipeRefresh() {
        swipeRefresh.setOnRefreshListener(() -> {
            webView.reload();
        });
        // Use the app's accent color scheme
        swipeRefresh.setColorSchemeResources(
                android.R.color.holo_blue_bright,
                android.R.color.holo_green_light,
                android.R.color.holo_orange_light,
                android.R.color.holo_red_light
        );
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
                    // Stop swipe refresh animation when page finishes loading
                    swipeRefresh.setRefreshing(false);
                }
            }

            @Override
            public boolean onShowFileChooser(WebView webView, android.webkit.ValueCallback<Uri[]> filePathCallback, FileChooserParams fileChooserParams) {
                // TODO: handle file chooser for uploads
                return false;
            }
        });

        // Enable cookies (needed for auth session)
        CookieManager.getInstance().setAcceptThirdPartyCookies(webView, true);
    }

    // --- Options Menu (refresh, switch server, clear cache) ---

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add(Menu.NONE, 1, 0, R.string.menu_refresh);
        menu.add(Menu.NONE, 2, 1, R.string.menu_server);
        menu.add(Menu.NONE, 3, 2, R.string.menu_clear_cache);
        return true;
    }

    @Override
    public boolean onOptionsItemSelected(MenuItem item) {
        switch (item.getItemId()) {
            case 1: // Refresh
                webView.reload();
                Toast.makeText(this, R.string.toast_refreshed, Toast.LENGTH_SHORT).show();
                return true;
            case 2: // Switch server
                showServerDialog();
                return true;
            case 3: // Clear cache and refresh
                webView.clearCache(true);
                webView.clearHistory();
                CookieManager.getInstance().removeAllCookies(null);
                Toast.makeText(this, R.string.toast_cache_cleared, Toast.LENGTH_SHORT).show();
                webView.reload();
                return true;
            default:
                return super.onOptionsItemSelected(item);
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
     * Show dialog for user to input ClawBench server URL.
     * Called on first launch or when user wants to change server.
     */
    private void showServerDialog() {
        View dialogView = getLayoutInflater().inflate(R.layout.dialog_server_url, null);
        EditText input = dialogView.findViewById(R.id.serverUrlInput);

        // Pre-fill with saved URL if exists
        String savedUrl = prefs.getString(KEY_SERVER_URL, "");
        input.setText(savedUrl);
        input.setSelection(input.getText().length());

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
                    loadUrl(url);
                })
                .setNegativeButton(R.string.dialog_negative, null)
                .setCancelable(false)
                .show();
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    getString(R.string.notification_channel_name),
                    NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription(getString(R.string.notification_channel_desc));
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) {
                nm.createNotificationChannel(channel);
            }
        }
    }

    @Override
    public void onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }

    @Override
    protected void onDestroy() {
        ChatKeepAliveService.stop(this);
        super.onDestroy();
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
            String serverUrl = prefs.getString(KEY_SERVER_URL, "");
            if (serverUrl.startsWith("https://")) {
                new AlertDialog.Builder(MainActivity.this)
                        .setTitle("SSL \u8bc1\u4e66\u9a8c\u8bc1\u5931\u8d25")
                        .setMessage("\u670d\u52a1\u5668\u4f7f\u7528\u81ea\u7b7e\u540d\u8bc1\u4e66\uff0c\u662f\u5426\u4fe1\u4efb\u8be5\u8bc1\u4e66\uff1f\n\n" + error.toString())
                        .setPositiveButton("\u4fe1\u4efb\u5e76\u7ee7\u7eed", (dialog, which) -> handler.proceed())
                        .setNegativeButton("\u53d6\u6d88", (dialog, which) -> handler.cancel())
                        .setCancelable(false)
                        .show();
            } else {
                handler.cancel();
            }
        }

        @Override
        public void onPageFinished(WebView view, String url) {
            super.onPageFinished(view, url);
            injectChatStateMonitor();
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            super.onReceivedError(view, request, error);
            if (request.isForMainFrame()) {
                Toast.makeText(MainActivity.this, R.string.error_connection_failed, Toast.LENGTH_SHORT).show();
            }
        }
    }

    /**
     * Inject JavaScript that monitors the ClawBench chat state.
     * - Intercepts fetch POST to /api/ai/chat -> starts keep-alive
     * - Intercepts EventSource SSE done/cancelled -> stops keep-alive
     */
    private void injectChatStateMonitor() {
        String js =
            "(function() {" +
            "  if (window.__clawbenchMonitorInstalled) return;" +
            "  window.__clawbenchMonitorInstalled = true;" +
            "" +
            "  // Intercept fetch to detect chat messages being sent" +
            "  var originalFetch = window.fetch;" +
            "  window.fetch = function() {" +
            "    var url = arguments[0];" +
            "    if (typeof url === 'string' && url.indexOf('/api/ai/chat') !== -1 && url.indexOf('/stream') === -1) {" +
            "      if (typeof AndroidNative !== 'undefined') AndroidNative.startKeepAlive();" +
            "    }" +
            "    return originalFetch.apply(this, arguments);" +
            "  };" +
            "" +
            "  // Intercept EventSource to detect SSE connection events" +
            "  var OrigES = window.EventSource;" +
            "  window.EventSource = function(url, config) {" +
            "    var es = new OrigES(url, config);" +
            "    if (typeof url === 'string' && url.indexOf('/api/ai/chat/stream') !== -1) {" +
            "      es.addEventListener('done', function() {" +
            "        setTimeout(function() {" +
            "          if (typeof AndroidNative !== 'undefined') AndroidNative.stopKeepAlive();" +
            "        }, 2000);" +
            "      });" +
            "      es.addEventListener('cancelled', function() {" +
            "        if (typeof AndroidNative !== 'undefined') AndroidNative.stopKeepAlive();" +
            "      });" +
            "    }" +
            "    return es;" +
            "  };" +
            "  window.EventSource.prototype = OrigES.prototype;" +
            "  window.EventSource.CONNECTING = OrigES.CONNECTING;" +
            "  window.EventSource.OPEN = OrigES.OPEN;" +
            "  window.EventSource.CLOSED = OrigES.CLOSED;" +
            "})();";

        webView.evaluateJavascript(js, null);
    }

    // --- JavaScript Interface ---

    public static class WebAppInterface {
        private final MainActivity activity;

        public WebAppInterface(MainActivity activity) {
            this.activity = activity;
        }

        @JavascriptInterface
        public void startKeepAlive() {
            activity.runOnUiThread(() -> ChatKeepAliveService.start(activity));
        }

        @JavascriptInterface
        public void stopKeepAlive() {
            activity.runOnUiThread(() -> ChatKeepAliveService.stop(activity));
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
    }
}
