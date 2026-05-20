package com.clawbench.app;

import android.annotation.SuppressLint;
import android.app.AlertDialog;
import android.content.Intent;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Bundle;
import android.util.Log;
import android.view.View;
import android.view.inputmethod.EditorInfo;
import android.webkit.CookieManager;
import android.webkit.SslErrorHandler;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebStorage;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.ProgressBar;
import android.widget.Toast;

import androidx.appcompat.app.AppCompatActivity;

/**
 * Sandbox browser Activity for testing forwarded ports.
 *
 * Runs in an independent process (":browser") to provide full Cookie/Storage
 * isolation from the main app. This allows login/authentication testing
 * without sharing session state with the main ClawBench WebView.
 *
 * Key features:
 * - Back/forward navigation within WebView
 * - URL bar with localhost-only navigation (external URLs → system browser)
 * - Refresh current page
 * - Clear browsing data (manual, with confirmation dialog)
 * - Data persists across sessions (not cleared on exit)
 * - Auto-accept SSL for localhost, prompt for others
 * - No AndroidNative bridge injected (clean browser environment)
 */
public class BrowserActivity extends AppCompatActivity {

    private static final String TAG = "ClawBench-Browser";

    private WebView webView;
    private EditText urlBar;
    private ProgressBar progressBar;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_browser);

        webView = findViewById(R.id.browserWebView);
        urlBar = findViewById(R.id.urlBar);
        progressBar = findViewById(R.id.progressBar);

        setupWebView();
        setupToolbar();

        // Load initial URL from Intent
        int port = getIntent().getIntExtra("port", 0);
        String protocol = getIntent().getStringExtra("protocol");
        if (port > 0 && protocol != null) {
            String initialUrl = protocol + "://localhost:" + port + "/";
            webView.loadUrl(initialUrl);
            urlBar.setText(initialUrl);
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

        // Allow mixed content (HTTP/HTTPS)
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);

        // Responsive layout
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(true);

        // Smooth scrolling
        webView.setOverScrollMode(WebView.OVER_SCROLL_NEVER);

        // Custom user agent to identify sandbox browser
        String ua = settings.getUserAgentString();
        settings.setUserAgentString(ua + " ClawBench-Browser/1.0");

        // NO AndroidNative bridge — this is a clean browser environment

        // WebView client with URL restriction and SSL handling
        webView.setWebViewClient(new SandboxWebViewClient());

        // Chrome client for progress bar
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onProgressChanged(WebView view, int newProgress) {
                if (newProgress < 100) {
                    progressBar.setVisibility(View.VISIBLE);
                    progressBar.setProgress(newProgress);
                } else {
                    progressBar.setVisibility(View.GONE);
                }
            }
        });

        // Accept third-party cookies
        CookieManager.getInstance().setAcceptThirdPartyCookies(webView, true);
    }

    private void setupToolbar() {
        // Back button: WebView history back, or close if no history
        findViewById(R.id.btnBack).setOnClickListener(v -> {
            if (webView.canGoBack()) {
                webView.goBack();
            } else {
                finish();
            }
        });

        // Refresh button
        findViewById(R.id.btnRefresh).setOnClickListener(v -> webView.reload());

        // Clear data button
        findViewById(R.id.btnClearData).setOnClickListener(v -> showClearDataDialog());

        // URL bar: navigate on Enter/Go
        urlBar.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_GO || actionId == EditorInfo.IME_ACTION_DONE) {
                navigateToUrl();
                return true;
            }
            return false;
        });

        // Also navigate on focus lost (if user taps away from URL bar)
        urlBar.setOnFocusChangeListener((v, hasFocus) -> {
            if (!hasFocus) {
                // Update URL bar with current page URL if user didn't edit
                // (prevents stale URL display after navigation)
            }
        });
    }

    /**
     * Navigate to the URL entered in the URL bar.
     * Only localhost URLs are loaded in the sandbox;
     * external URLs are opened in the system browser.
     */
    private void navigateToUrl() {
        String input = urlBar.getText().toString().trim();
        if (input.isEmpty()) return;

        // Ensure it has a scheme
        if (!input.startsWith("http://") && !input.startsWith("https://")) {
            input = "http://" + input;
        }

        try {
            Uri uri = Uri.parse(input);
            String host = uri.getHost();

            if ("localhost".equals(host) || "127.0.0.1".equals(host)) {
                webView.loadUrl(input);
            } else {
                // External URL: open in system browser, not in sandbox
                startActivity(new Intent(Intent.ACTION_VIEW, uri));
            }
        } catch (Exception e) {
            AppLog.w(TAG, "Invalid URL: " + input, e);
        }

        // Hide keyboard
        urlBar.clearFocus();
    }

    /**
     * Show confirmation dialog for clearing browsing data.
     * Data is preserved by default; user must explicitly clear it.
     */
    private void showClearDataDialog() {
        new AlertDialog.Builder(this)
                .setTitle(R.string.browser_clear_title)
                .setMessage(R.string.browser_clear_message)
                .setPositiveButton(R.string.browser_clear_positive, (dialog, which) -> clearBrowsingData())
                .setNegativeButton(R.string.browser_clear_negative, null)
                .show();
    }

    /**
     * Clear all browsing data: cookies, WebStorage, cache, form data.
     * Then reload the current page to reflect the clean state.
     */
    private void clearBrowsingData() {
        CookieManager.getInstance().removeAllCookies(null);
        WebStorage.getInstance().deleteAllData();
        webView.clearCache(true);
        webView.clearFormData();
        webView.clearHistory();

        Toast.makeText(this, R.string.browser_clear_done, Toast.LENGTH_SHORT).show();

        // Reload current page to show logged-out state
        if (webView.getUrl() != null) {
            webView.reload();
        }
    }

    @Override
    protected void onPause() {
        super.onPause();
        pauseWebView();
    }

    @Override
    protected void onResume() {
        super.onResume();
        resumeWebView();
    }

    /** Pause WebView rendering and JS timers to release CPU/GPU resources. */
    void pauseWebView() {
        webView.onPause();
        webView.pauseTimers();
    }

    /** Resume WebView rendering and JS timers when returning to foreground. */
    void resumeWebView() {
        webView.onResume();
        webView.resumeTimers();
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
        // Do NOT clear data here — it should persist across sessions.
        // Only release WebView resources.
        webView.loadUrl("about:blank");
        webView.destroy();
        super.onDestroy();
    }

    // --- WebView Client ---

    private class SandboxWebViewClient extends WebViewClient {

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            Uri url = request.getUrl();
            String host = url.getHost();

            // Only allow localhost URLs in the sandbox
            if ("localhost".equals(host) || "127.0.0.1".equals(host)) {
                return false; // Load in sandbox WebView
            }

            // External URLs → system browser
            startActivity(new Intent(Intent.ACTION_VIEW, url));
            return true;
        }

        @Override
        public void onPageFinished(WebView view, String url) {
            // Update URL bar to reflect actual page URL
            urlBar.setText(url);
        }

        @Override
        public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
            String host = null;
            String currentUrl = view.getUrl();
            if (currentUrl != null) {
                host = Uri.parse(currentUrl).getHost();
            }

            // Auto-accept SSL for localhost (self-signed certs on forwarded ports)
            if ("localhost".equals(host) || "127.0.0.1".equals(host)) {
                handler.proceed();
                return;
            }

            // Non-localhost: prompt user before accepting
            new AlertDialog.Builder(BrowserActivity.this)
                    .setTitle(R.string.browser_ssl_title)
                    .setMessage(R.string.browser_ssl_message)
                    .setPositiveButton(R.string.browser_ssl_positive, (dialog, which) -> handler.proceed())
                    .setNegativeButton(R.string.browser_ssl_negative, (dialog, which) -> handler.cancel())
                    .setCancelable(false)
                    .show();
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            super.onReceivedError(view, request, error);
            if (request.isForMainFrame()) {
                Toast.makeText(BrowserActivity.this, R.string.error_connection_failed, Toast.LENGTH_SHORT).show();
            }
        }
    }
}
